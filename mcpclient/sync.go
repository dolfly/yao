package mcpclient

import (
	"encoding/json"
	"fmt"

	"github.com/yaoapp/gou/mcp"
	mcpTypes "github.com/yaoapp/gou/mcp/types"
)

// runtimeID builds the runtime ID for registering into mcp.clients.
// Dynamic clients get an owner prefix to avoid collision with builtin IDs.
func runtimeID(c *Client) string {
	switch c.Owner.Type {
	case "user":
		return "u" + c.Owner.ID + "." + c.ID
	case "team":
		return "t" + c.Owner.ID + "." + c.ID
	case "assistant":
		return "a" + c.Owner.ID + "." + c.ID
	default:
		return "s." + c.ID
	}
}

// ensureClient makes sure the MCP client is registered in the runtime.
// Builtin clients are managed by engine.Load and skipped here.
func ensureClient(c *Client) error {
	if c.Source == ClientSourceBuiltIn {
		return nil
	}
	if !c.Enabled {
		return nil
	}

	rid := c.RuntimeID
	if rid == "" {
		rid = runtimeID(c)
	}

	if mcp.Exists(rid) {
		return nil
	}

	dslJSON, err := json.Marshal(c.ClientDSL)
	if err != nil {
		return fmt.Errorf("ensureClient %s: marshal DSL: %w", c.ID, err)
	}

	clientType := c.ClientDSL.Type
	_, err = mcp.LoadClientSourceWithType(string(dslJSON), rid, clientType)
	if err != nil {
		return fmt.Errorf("ensureClient %s: LoadClientSourceWithType: %w", c.ID, err)
	}

	return nil
}

// unloadClient removes the client from the runtime.
func unloadClient(c *Client) {
	if c.Source == ClientSourceBuiltIn {
		return
	}
	rid := c.RuntimeID
	if rid == "" {
		rid = runtimeID(c)
	}
	mcp.UnloadClient(rid)
}

// importFromClients scans existing MCP clients loaded by engine.Load
// and imports them as builtin entries into the Registry store.
// If a store record with the same ID already exists (dynamic), it is not overwritten.
func importFromClients(r *Registry) error {
	ids := mcp.ListClients()
	for _, id := range ids {
		if r.store.Has(storeKey(id)) {
			continue
		}

		cl := clientFromRuntime(id)
		if cl == nil {
			continue
		}

		m, err := clientToMap(cl)
		if err != nil {
			continue
		}
		sk := storeKey(id)
		_ = r.store.Set(sk, m, 0)
		if r.cache != nil {
			_ = r.cache.Set(sk, m, 0)
		}
		_ = indexAdd(r.store, r.cache, id)
	}
	return nil
}

// clientFromRuntime builds a Client from a runtime mcp.Client interface.
// Uses Info() and GetMetaInfo() since full ClientDSL is not exposed.
func clientFromRuntime(id string) *Client {
	defer func() { recover() }()

	mcpClient := mcp.GetClient(id)
	if mcpClient == nil {
		return nil
	}

	info := mcpClient.Info()
	if info == nil {
		return nil
	}

	meta := mcpClient.GetMetaInfo()

	name := info.Name
	if name == "" {
		name = id
	}

	return &Client{
		ClientDSL: mcpTypes.ClientDSL{
			ID:        id,
			Name:      name,
			Type:      info.Type,
			Transport: info.Transport,
			MetaInfo:  meta,
		},
		RuntimeID: id,
		Enabled:   true,
		Status:    "connected",
		Source:    ClientSourceBuiltIn,
		Owner:     ClientOwner{Type: "system"},
	}
}
