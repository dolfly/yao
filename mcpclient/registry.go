package mcpclient

import (
	"fmt"
	"strings"
	"sync"

	"github.com/yaoapp/gou/mcp"
	"github.com/yaoapp/gou/store"
)

// Global is the singleton MCP Client Registry.
var Global *Registry

// Registry manages MCP clients with CRUD, persistence, cache and runtime sync.
type Registry struct {
	store store.Store
	cache store.Store
	mu    sync.RWMutex
}

// Init initializes the global Registry.
// Must be called after store.Load and mcp.Load.
func Init() error {
	s, err := store.Get("__yao.store")
	if err != nil {
		return fmt.Errorf("mcpclient.Init: %w", err)
	}
	c, _ := store.Get("__yao.cache")

	r := &Registry{store: s, cache: c}
	Global = r

	if err := importFromClients(r); err != nil {
		return fmt.Errorf("mcpclient.Init importFromClients: %w", err)
	}

	return nil
}

// Get retrieves a client by ID. Lazily ensures its runtime client is registered.
func (r *Registry) Get(id string) (*Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, err := storeGet(r.store, r.cache, id)
	if err != nil {
		return nil, err
	}

	_ = ensureClient(c)
	return c, nil
}

// Create adds a new client. Persists, caches, registers runtime, and updates index.
func (r *Registry) Create(c *Client) (*Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c.ID == "" {
		return nil, fmt.Errorf("client id is required")
	}
	if r.store.Has(storeKey(c.ID)) {
		return nil, fmt.Errorf("client %s already exists", c.ID)
	}

	if c.Source == "" {
		c.Source = ClientSourceDynamic
	}
	if c.RuntimeID == "" {
		c.RuntimeID = runtimeID(c)
	}
	if c.Status == "" {
		c.Status = "unconfigured"
	}

	if err := storeSet(r.store, r.cache, c); err != nil {
		return nil, err
	}
	if err := indexAdd(r.store, r.cache, c.ID); err != nil {
		return nil, err
	}

	if c.Enabled {
		_ = ensureClient(c)
	}

	return c, nil
}

// Update modifies an existing client. Hot-replaces the runtime client.
func (r *Registry) Update(id string, c *Client) (*Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	old, err := storeGet(r.store, r.cache, id)
	if err != nil {
		return nil, err
	}

	c.ID = id
	if c.Source == "" {
		c.Source = old.Source
	}
	if c.RuntimeID == "" {
		c.RuntimeID = old.RuntimeID
	}
	if c.Owner == (ClientOwner{}) {
		c.Owner = old.Owner
	}

	unloadClient(old)

	if err := storeSet(r.store, r.cache, c); err != nil {
		return nil, err
	}

	if c.Enabled {
		_ = ensureClient(c)
	}

	return c, nil
}

// Delete removes a client by ID. Unloads runtime, deletes store/cache/index.
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, err := storeGet(r.store, r.cache, id)
	if err != nil {
		return err
	}

	unloadClient(c)

	if err := storeDel(r.store, r.cache, id); err != nil {
		return err
	}
	return indexRemove(r.store, r.cache, id)
}

// List returns clients matching the filter.
func (r *Registry) List(filter *ClientFilter) ([]Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids, err := indexGet(r.store, r.cache)
	if err != nil {
		return nil, err
	}

	var result []Client
	for _, id := range ids {
		c, err := storeGet(r.store, r.cache, id)
		if err != nil {
			continue
		}
		if filter != nil && !matchFilter(c, filter) {
			continue
		}
		result = append(result, *c)
	}
	return result, nil
}

// Reload re-reads all clients from persistent store and rebuilds cache + runtime.
func (r *Registry) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids, err := indexGet(r.store, nil)
	if err != nil {
		return err
	}

	for _, id := range ids {
		c, err := storeGet(r.store, nil, id)
		if err != nil {
			continue
		}
		m, err := clientToMap(c)
		if err != nil {
			continue
		}
		if r.cache != nil {
			r.cache.Set(storeKey(id), m, 0)
		}
		if c.Source == ClientSourceDynamic && c.Enabled {
			_ = ensureClient(c)
		}
	}
	return nil
}

// GetMCPClient returns the runtime mcp.Client for a given registry ID.
func (r *Registry) GetMCPClient(id string) (mcp.Client, error) {
	c, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	rid := c.RuntimeID
	if rid == "" {
		rid = runtimeID(c)
	}

	defer func() { recover() }()
	client := mcp.GetClient(rid)
	if client == nil {
		return nil, fmt.Errorf("runtime mcp client %s not found", rid)
	}
	return client, nil
}

func matchFilter(c *Client, f *ClientFilter) bool {
	src := f.Source
	if src == "" {
		src = ClientSourceDynamic
	}
	if src != ClientSourceAll && c.Source != src {
		return false
	}

	if f.Owner != nil {
		if f.Owner.Type != "" && c.Owner.Type != f.Owner.Type {
			return false
		}
		if f.Owner.ID != "" && c.Owner.ID != f.Owner.ID {
			return false
		}
	}

	if f.Enabled != nil && c.Enabled != *f.Enabled {
		return false
	}

	if f.Transport != nil && c.ClientDSL.Transport != *f.Transport {
		return false
	}

	if f.Type != nil && c.ClientDSL.Type != *f.Type {
		return false
	}

	if f.Keyword != "" {
		kw := strings.ToLower(f.Keyword)
		if !strings.Contains(strings.ToLower(c.ClientDSL.Name), kw) &&
			!strings.Contains(strings.ToLower(c.ID), kw) &&
			!strings.Contains(strings.ToLower(c.ClientDSL.Label), kw) {
			return false
		}
	}

	return true
}
