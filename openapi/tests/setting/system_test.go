package setting_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yaoapp/yao/openapi"
	"github.com/yaoapp/yao/openapi/tests/testutils"
)

func baseURL() string {
	if openapi.Server != nil && openapi.Server.Config != nil {
		return openapi.Server.Config.BaseURL
	}
	return ""
}

// TestSystemInfo verifies GET /setting/system returns the expected structure.
func TestSystemInfo(t *testing.T) {
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	client := testutils.RegisterTestClient(t, "Setting System Test", []string{"https://localhost/callback"})
	defer testutils.CleanupTestClient(t, client.ClientID)
	token := testutils.ObtainAccessToken(t, serverURL, client.ClientID, client.ClientSecret, "https://localhost/callback", "openid profile")

	req, err := http.NewRequest("GET", serverURL+baseURL()+"/setting/system", nil)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(t, err)

	// Top-level keys
	assert.Contains(t, body, "app")
	assert.Contains(t, body, "deployment")
	assert.Contains(t, body, "server")
	assert.Contains(t, body, "client")
	assert.Contains(t, body, "environment")
	assert.Contains(t, body, "technical")

	// app sub-fields
	app, ok := body["app"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, app["name"])
	assert.NotEmpty(t, app["version"])

	// server sub-fields
	server, ok := body["server"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, server["version"])

	// technical sub-fields
	tech, ok := body["technical"].(map[string]interface{})
	assert.True(t, ok)
	assert.NotEmpty(t, tech["listen"])
	assert.NotEmpty(t, tech["db_driver"])
	assert.NotEmpty(t, tech["session_store"])
}

// TestSystemInfoUnauthenticated verifies 401 when no token is provided.
func TestSystemInfoUnauthenticated(t *testing.T) {
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	req, err := http.NewRequest("GET", serverURL+baseURL()+"/setting/system", nil)
	assert.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestSystemCheckUpdate verifies POST /setting/system/check-update returns has_update.
func TestSystemCheckUpdate(t *testing.T) {
	serverURL := testutils.Prepare(t)
	defer testutils.Clean()

	client := testutils.RegisterTestClient(t, "Setting CheckUpdate Test", []string{"https://localhost/callback"})
	defer testutils.CleanupTestClient(t, client.ClientID)
	token := testutils.ObtainAccessToken(t, serverURL, client.ClientID, client.ClientSecret, "https://localhost/callback", "openid profile")

	req, err := http.NewRequest("POST", serverURL+baseURL()+"/setting/system/check-update", nil)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(t, err)

	_, exists := body["has_update"]
	assert.True(t, exists, "response must contain has_update field")
}
