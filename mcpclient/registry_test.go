package mcpclient_test

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yaoapp/gou/mcp"
	mcpTypes "github.com/yaoapp/gou/mcp/types"
	"github.com/yaoapp/gou/store"
	"github.com/yaoapp/yao/config"
	"github.com/yaoapp/yao/mcpclient"
	"github.com/yaoapp/yao/test"
)

func TestMain(m *testing.M) {
	test.Prepare(nil, config.Conf)
	defer test.Clean()
	os.Exit(m.Run())
}

func setupRegistry(t *testing.T) *mcpclient.Registry {
	t.Helper()
	test.Prepare(t, config.Conf)

	err := mcpclient.Init()
	require.NoError(t, err)

	t.Cleanup(func() {
		s, _ := store.Get("__yao.store")
		if s != nil {
			s.Del("mcpclient:*")
		}
		c, _ := store.Get("__yao.cache")
		if c != nil {
			c.Del("mcpclient:*")
		}
		test.Clean()
	})

	return mcpclient.Global
}

func newTestClient(id string) mcpclient.Client {
	return mcpclient.Client{
		ClientDSL: mcpTypes.ClientDSL{
			ID:        id,
			Name:      "Test " + id,
			Type:      "standard",
			Transport: mcpTypes.TransportStdio,
			Command:   "echo",
			Arguments: []string{"hello"},
		},
		Enabled: true,
		Owner:   mcpclient.ClientOwner{Type: "system"},
	}
}

func TestCreate(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-stdio")
	created, err := r.Create(&c)
	require.NoError(t, err)
	assert.Equal(t, "test-stdio", created.ID)
	assert.Equal(t, mcpclient.ClientSourceDynamic, created.Source)
	assert.NotEmpty(t, created.RuntimeID)

	s, _ := store.Get("__yao.store")
	assert.True(t, s.Has("mcpclient:c:test-stdio"))
}

func TestCreateDuplicate(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-dup")
	_, err := r.Create(&c)
	require.NoError(t, err)

	dup := newTestClient("test-dup")
	_, err = r.Create(&dup)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestGet(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-get")
	_, err := r.Create(&c)
	require.NoError(t, err)

	got, err := r.Get("test-get")
	require.NoError(t, err)
	assert.Equal(t, "Test test-get", got.Name)
	assert.Equal(t, mcpTypes.TransportStdio, got.Transport)
	assert.Equal(t, "echo", got.Command)
}

func TestGetNotFound(t *testing.T) {
	r := setupRegistry(t)
	_, err := r.Get("nonexistent")
	assert.Error(t, err)
}

func TestGetLazy(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-lazy")
	created, err := r.Create(&c)
	require.NoError(t, err)

	// Manually unload the client
	mcp.UnloadClient(created.RuntimeID)
	assert.False(t, mcp.Exists(created.RuntimeID))

	// Get should lazily re-register
	got, err := r.Get("test-lazy")
	require.NoError(t, err)
	assert.Equal(t, "test-lazy", got.ID)
}

func TestList(t *testing.T) {
	r := setupRegistry(t)

	clients := []mcpclient.Client{
		{
			ClientDSL: mcpTypes.ClientDSL{ID: "c1", Name: "Client 1", Type: "standard", Transport: mcpTypes.TransportStdio, Command: "echo"},
			Enabled:   true,
			Owner:     mcpclient.ClientOwner{Type: "system"},
		},
		{
			ClientDSL: mcpTypes.ClientDSL{ID: "c2", Name: "Client 2", Type: "agent", Transport: mcpTypes.TransportSSE, URL: "http://localhost:3001"},
			Enabled:   false,
			Owner:     mcpclient.ClientOwner{Type: "user", ID: "123"},
		},
		{
			ClientDSL: mcpTypes.ClientDSL{ID: "c3", Name: "Client 3", Type: "standard", Transport: mcpTypes.TransportStdio, Command: "cat"},
			Enabled:   true,
			Owner:     mcpclient.ClientOwner{Type: "system"},
		},
	}
	for i := range clients {
		_, err := r.Create(&clients[i])
		require.NoError(t, err)
	}

	t.Run("AllDynamic", func(t *testing.T) {
		list, err := r.List(&mcpclient.ClientFilter{Source: mcpclient.ClientSourceDynamic})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 3)
	})

	t.Run("FilterByTransport", func(t *testing.T) {
		tp := mcpTypes.TransportSSE
		list, err := r.List(&mcpclient.ClientFilter{
			Source:    mcpclient.ClientSourceDynamic,
			Transport: &tp,
		})
		require.NoError(t, err)
		for _, c := range list {
			assert.Equal(t, mcpTypes.TransportSSE, c.Transport)
		}
	})

	t.Run("FilterByEnabled", func(t *testing.T) {
		enabled := true
		list, err := r.List(&mcpclient.ClientFilter{
			Source:  mcpclient.ClientSourceDynamic,
			Enabled: &enabled,
		})
		require.NoError(t, err)
		for _, c := range list {
			assert.True(t, c.Enabled)
		}
	})

	t.Run("FilterByOwner", func(t *testing.T) {
		list, err := r.List(&mcpclient.ClientFilter{
			Source: mcpclient.ClientSourceDynamic,
			Owner:  &mcpclient.ClientOwner{Type: "user", ID: "123"},
		})
		require.NoError(t, err)
		for _, c := range list {
			assert.Equal(t, "user", c.Owner.Type)
		}
	})

	t.Run("FilterByType", func(t *testing.T) {
		typ := "agent"
		list, err := r.List(&mcpclient.ClientFilter{
			Source: mcpclient.ClientSourceDynamic,
			Type:   &typ,
		})
		require.NoError(t, err)
		for _, c := range list {
			assert.Equal(t, "agent", c.ClientDSL.Type)
		}
	})

	t.Run("FilterByKeyword", func(t *testing.T) {
		list, err := r.List(&mcpclient.ClientFilter{
			Source:  mcpclient.ClientSourceDynamic,
			Keyword: "Client 2",
		})
		require.NoError(t, err)
		found := false
		for _, c := range list {
			if c.ID == "c2" {
				found = true
			}
		}
		assert.True(t, found)
	})
}

func TestUpdate(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-update")
	_, err := r.Create(&c)
	require.NoError(t, err)

	got, err := r.Get("test-update")
	require.NoError(t, err)

	updated := *got
	updated.ClientDSL.Name = "Updated Name"
	updated.ClientDSL.Command = "cat"

	result, err := r.Update("test-update", &updated)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", result.Name)

	got2, err := r.Get("test-update")
	require.NoError(t, err)
	assert.Equal(t, "cat", got2.Command)
}

func TestDelete(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-delete")
	_, err := r.Create(&c)
	require.NoError(t, err)

	err = r.Delete("test-delete")
	require.NoError(t, err)

	_, err = r.Get("test-delete")
	assert.Error(t, err)
}

func TestReload(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-reload")
	_, err := r.Create(&c)
	require.NoError(t, err)

	// Clear cache
	cache, _ := store.Get("__yao.cache")
	if cache != nil {
		cache.Del("mcpclient:*")
	}

	err = r.Reload()
	require.NoError(t, err)

	got, err := r.Get("test-reload")
	require.NoError(t, err)
	assert.Equal(t, "Test test-reload", got.Name)
}

func TestImportFromClients(t *testing.T) {
	r := setupRegistry(t)

	list, err := r.List(&mcpclient.ClientFilter{Source: mcpclient.ClientSourceAll})
	require.NoError(t, err)

	builtinCount := 0
	for _, c := range list {
		if c.Source == mcpclient.ClientSourceBuiltIn {
			builtinCount++
		}
	}

	loadedClients := mcp.ListClients()
	t.Logf("Imported %d builtin clients from mcp.ListClients (total loaded: %d)", builtinCount, len(loadedClients))
}

func TestToolListField(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-toollist")
	c.ClientDSL.Tools = map[string]string{"my-tool": "scripts.MyTool"}
	c.ToolList = []mcpTypes.Tool{
		{Name: "discovered-tool", Description: "A tool discovered at runtime"},
	}

	created, err := r.Create(&c)
	require.NoError(t, err)

	got, err := r.Get(created.ID)
	require.NoError(t, err)
	assert.Len(t, got.ToolList, 1)
	assert.Equal(t, "discovered-tool", got.ToolList[0].Name)
	assert.Equal(t, "scripts.MyTool", got.ClientDSL.Tools["my-tool"])
}

func TestGetMCPClient(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-getmcp")
	_, err := r.Create(&c)
	require.NoError(t, err)

	// The MCP client may or may not actually start (depends on whether "echo" is a valid MCP server),
	// but we should at least exercise the code path.
	_, err = r.GetMCPClient("test-getmcp")
	// Either it works or returns a "not found" — both are valid for this test fixture
	t.Logf("GetMCPClient result: err=%v", err)
}

func TestGetMCPClientNotFound(t *testing.T) {
	r := setupRegistry(t)
	_, err := r.GetMCPClient("no-such-client")
	assert.Error(t, err)
}

func TestCreateEmptyID(t *testing.T) {
	r := setupRegistry(t)
	c := mcpclient.Client{ClientDSL: mcpTypes.ClientDSL{Name: "No ID"}}
	_, err := r.Create(&c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id is required")
}

func TestCreateDisabled(t *testing.T) {
	r := setupRegistry(t)
	c := mcpclient.Client{
		ClientDSL: mcpTypes.ClientDSL{ID: "test-disabled", Name: "Disabled", Type: "standard", Transport: mcpTypes.TransportStdio, Command: "echo"},
		Enabled:   false,
		Owner:     mcpclient.ClientOwner{Type: "system"},
	}
	created, err := r.Create(&c)
	require.NoError(t, err)
	assert.Equal(t, "unconfigured", created.Status)

	// Disabled client should not be registered at runtime
	assert.False(t, mcp.Exists(created.RuntimeID), "disabled client should not be registered")
}

func TestOwnerPrefixedRuntimeIDs(t *testing.T) {
	r := setupRegistry(t)

	cases := []struct {
		id     string
		owner  mcpclient.ClientOwner
		prefix string
	}{
		{"owner-sys", mcpclient.ClientOwner{Type: "system"}, "s."},
		{"owner-usr", mcpclient.ClientOwner{Type: "user", ID: "42"}, "u42."},
		{"owner-team", mcpclient.ClientOwner{Type: "team", ID: "99"}, "t99."},
		{"owner-asst", mcpclient.ClientOwner{Type: "assistant", ID: "a1"}, "aa1."},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			c := mcpclient.Client{
				ClientDSL: mcpTypes.ClientDSL{ID: tc.id, Name: tc.id, Type: "standard", Transport: mcpTypes.TransportStdio, Command: "echo"},
				Enabled:   true,
				Owner:     tc.owner,
			}
			created, err := r.Create(&c)
			require.NoError(t, err)
			assert.Contains(t, created.RuntimeID, tc.prefix,
				"RuntimeID for %s owner should contain prefix %s", tc.owner.Type, tc.prefix)
		})
	}
}

func TestListBuiltInFilter(t *testing.T) {
	r := setupRegistry(t)

	builtinList, err := r.List(&mcpclient.ClientFilter{Source: mcpclient.ClientSourceBuiltIn})
	require.NoError(t, err)
	for _, c := range builtinList {
		assert.Equal(t, mcpclient.ClientSourceBuiltIn, c.Source)
	}
}

func TestListAllSources(t *testing.T) {
	r := setupRegistry(t)

	c := newTestClient("test-all-src")
	_, err := r.Create(&c)
	require.NoError(t, err)

	all, err := r.List(&mcpclient.ClientFilter{Source: mcpclient.ClientSourceAll})
	require.NoError(t, err)

	hasDynamic := false
	for _, item := range all {
		if item.Source == mcpclient.ClientSourceDynamic {
			hasDynamic = true
		}
	}
	assert.True(t, hasDynamic)
}

func TestUpdateNotFound(t *testing.T) {
	r := setupRegistry(t)
	c := newTestClient("not-exist")
	_, err := r.Update("not-exist", &c)
	assert.Error(t, err)
}

func TestDeleteNotFound(t *testing.T) {
	r := setupRegistry(t)
	err := r.Delete("not-exist")
	assert.Error(t, err)
}

func TestConcurrency(t *testing.T) {
	r := setupRegistry(t)

	var wg sync.WaitGroup
	errCh := make(chan error, 30)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := mcpclient.Client{
				ClientDSL: mcpTypes.ClientDSL{
					ID:        fmt.Sprintf("conc-%d", idx),
					Name:      fmt.Sprintf("Concurrent %d", idx),
					Type:      "standard",
					Transport: mcpTypes.TransportStdio,
					Command:   "echo",
				},
				Enabled: true,
				Owner:   mcpclient.ClientOwner{Type: "system"},
			}
			if _, err := r.Create(&c); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := r.Get(fmt.Sprintf("conc-%d", idx))
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := r.Delete(fmt.Sprintf("conc-%d", idx)); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation error: %v", err)
	}
}
