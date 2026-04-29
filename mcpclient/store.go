package mcpclient

import (
	"encoding/json"
	"fmt"

	"github.com/yaoapp/gou/store"
)

const (
	keyPrefix = "mcpclient:c:"
	indexKey  = "mcpclient:index"
)

func storeKey(id string) string { return keyPrefix + id }

func clientToMap(c *Client) (map[string]interface{}, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func mapToClient(m map[string]interface{}) (*Client, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var c Client
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func storeGet(s, c store.Store, id string) (*Client, error) {
	sk := storeKey(id)

	if c != nil {
		if val, ok := c.Get(sk); ok {
			if m, ok := val.(map[string]interface{}); ok {
				return mapToClient(m)
			}
		}
	}

	val, ok := s.Get(sk)
	if !ok {
		return nil, fmt.Errorf("client %s not found", id)
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("client %s: unexpected store type %T", id, val)
	}

	cl, err := mapToClient(m)
	if err != nil {
		return nil, err
	}

	if c != nil {
		c.Set(sk, m, 0)
	}
	return cl, nil
}

func storeSet(s, c store.Store, cl *Client) error {
	m, err := clientToMap(cl)
	if err != nil {
		return err
	}
	sk := storeKey(cl.ID)
	if err := s.Set(sk, m, 0); err != nil {
		return err
	}
	if c != nil {
		c.Set(sk, m, 0)
	}
	return nil
}

func storeDel(s, c store.Store, id string) error {
	sk := storeKey(id)
	if err := s.Del(sk); err != nil {
		return err
	}
	if c != nil {
		c.Del(sk)
	}
	return nil
}

func indexGet(s, c store.Store) ([]string, error) {
	var raw interface{}
	var ok bool

	if c != nil {
		raw, ok = c.Get(indexKey)
	}
	if !ok {
		raw, ok = s.Get(indexKey)
		if !ok {
			return nil, nil
		}
		if c != nil {
			c.Set(indexKey, raw, 0)
		}
	}

	switch v := raw.(type) {
	case []interface{}:
		keys := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				keys = append(keys, str)
			}
		}
		return keys, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("unexpected index type %T", raw)
	}
}

func indexSet(s, c store.Store, ids []string) error {
	iface := make([]interface{}, len(ids))
	for i, k := range ids {
		iface[i] = k
	}
	if err := s.Set(indexKey, iface, 0); err != nil {
		return err
	}
	if c != nil {
		c.Set(indexKey, iface, 0)
	}
	return nil
}

func indexAdd(s, c store.Store, id string) error {
	ids, err := indexGet(s, c)
	if err != nil {
		return err
	}
	for _, k := range ids {
		if k == id {
			return nil
		}
	}
	return indexSet(s, c, append(ids, id))
}

func indexRemove(s, c store.Store, id string) error {
	ids, err := indexGet(s, c)
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(ids))
	for _, k := range ids {
		if k != id {
			filtered = append(filtered, k)
		}
	}
	return indexSet(s, c, filtered)
}

func storeCleanAll(s, c store.Store) {
	_ = s.Del(keyPrefix + "*")
	_ = s.Del(indexKey)
	if c != nil {
		_ = c.Del(keyPrefix + "*")
		_ = c.Del(indexKey)
	}
}
