package llmprovider

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/yaoapp/gou/store"
)

const (
	keyPrefix = "llmprovider:p:"
	indexKey  = "llmprovider:index"
	maskChars = 4
	encPrefix = "enc:"
)

func storeKey(key string) string { return keyPrefix + key }

// providerToMap converts Provider to map[string]interface{} for store.Set.
// Encrypts APIKey before writing.
func providerToMap(p *Provider, encKey string) (map[string]interface{}, error) {
	cp := *p
	if cp.APIKey != "" && encKey != "" {
		encrypted, err := encryptString(cp.APIKey, encKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt api_key: %w", err)
		}
		cp.APIKey = encPrefix + encrypted
	}

	raw, err := json.Marshal(cp)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// mapToProvider converts map[string]interface{} from store.Get back to Provider.
// Decrypts APIKey after reading.
func mapToProvider(m map[string]interface{}, encKey string) (*Provider, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var p Provider
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if strings.HasPrefix(p.APIKey, encPrefix) && encKey != "" {
		decrypted, err := decryptString(strings.TrimPrefix(p.APIKey, encPrefix), encKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt api_key: %w", err)
		}
		p.APIKey = decrypted
	}
	return &p, nil
}

// maskAPIKey returns a masked version of the API key for display.
func maskAPIKey(key string) string {
	if len(key) <= maskChars {
		return strings.Repeat("*", len(key))
	}
	return strings.Repeat("*", len(key)-maskChars) + key[len(key)-maskChars:]
}

// storeGet reads a provider from cache first, then persistent store.
func storeGet(s, c store.Store, key, encKey string) (*Provider, error) {
	sk := storeKey(key)

	if c != nil {
		if val, ok := c.Get(sk); ok {
			if m, ok := val.(map[string]interface{}); ok {
				return mapToProvider(m, encKey)
			}
		}
	}

	val, ok := s.Get(sk)
	if !ok {
		return nil, fmt.Errorf("provider %s not found", key)
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("provider %s: unexpected store type %T", key, val)
	}

	p, err := mapToProvider(m, encKey)
	if err != nil {
		return nil, err
	}

	if c != nil {
		c.Set(sk, m, 0)
	}
	return p, nil
}

// storeSet writes a provider to both persistent store and cache.
func storeSet(s, c store.Store, p *Provider, encKey string) error {
	m, err := providerToMap(p, encKey)
	if err != nil {
		return err
	}
	sk := storeKey(p.Key)
	if err := s.Set(sk, m, 0); err != nil {
		return err
	}
	if c != nil {
		c.Set(sk, m, 0)
	}
	return nil
}

// storeDel removes a provider from both persistent store and cache.
func storeDel(s, c store.Store, key string) error {
	sk := storeKey(key)
	if err := s.Del(sk); err != nil {
		return err
	}
	if c != nil {
		c.Del(sk)
	}
	return nil
}

// indexGet returns all provider keys from the index.
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

// indexSet writes the full index to both stores.
func indexSet(s, c store.Store, keys []string) error {
	iface := make([]interface{}, len(keys))
	for i, k := range keys {
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

// indexAdd appends a key to the index if not present.
func indexAdd(s, c store.Store, key string) error {
	keys, err := indexGet(s, c)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if k == key {
			return nil
		}
	}
	return indexSet(s, c, append(keys, key))
}

// indexRemove removes a key from the index.
func indexRemove(s, c store.Store, key string) error {
	keys, err := indexGet(s, c)
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(keys))
	for _, k := range keys {
		if k != key {
			filtered = append(filtered, k)
		}
	}
	return indexSet(s, c, filtered)
}

// --- AES-256-GCM encryption helpers ---

func deriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

func encryptString(plaintext, secret string) (string, error) {
	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptString(encoded, secret string) (string, error) {
	key := deriveKey(secret)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// storeCleanAll removes all llmprovider keys (for testing cleanup).
func storeCleanAll(s, c store.Store) {
	_ = s.Del(keyPrefix + "*")
	_ = s.Del(indexKey)
	if c != nil {
		_ = c.Del(keyPrefix + "*")
		_ = c.Del(indexKey)
	}
}
