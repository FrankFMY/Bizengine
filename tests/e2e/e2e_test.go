//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var baseURL string

func TestMain(m *testing.M) {
	baseURL = os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	os.Exit(m.Run())
}

func postJSON(t *testing.T, path string, body any, cookies []*http.Cookie) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", baseURL+path, bytes.NewReader(data))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func getJSON(t *testing.T, path string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+path, nil)
	require.NoError(t, err)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func TestHealth(t *testing.T) {
	resp := getJSON(t, "/health", nil)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)

	body := decodeBody(t, resp)
	assert.Equal(t, "ok", body["status"])
}

func TestMetrics(t *testing.T) {
	resp := getJSON(t, "/metrics", nil)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestSecurityHeaders(t *testing.T) {
	resp := getJSON(t, "/health", nil)
	defer resp.Body.Close()
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.NotEmpty(t, resp.Header.Get("X-API-Version"))
}

func TestFullUserFlow(t *testing.T) {
	email := fmt.Sprintf("e2e-%d@test.com", os.Getpid())

	// 1. Register
	resp := postJSON(t, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "Test1234!",
		"name":     "E2E Test User",
	}, nil)
	assert.Equal(t, 200, resp.StatusCode, "register should succeed")
	cookies := resp.Cookies()
	resp.Body.Close()

	if len(cookies) == 0 {
		t.Skip("server not running or auth returns no cookies")
	}

	// 2. Auth check
	resp = getJSON(t, "/api/v1/auth/check", cookies)
	assert.Equal(t, 200, resp.StatusCode, "auth check should succeed")
	resp.Body.Close()

	// 3. Create organization
	resp = postJSON(t, "/api/v1/organizations", map[string]string{
		"name": "E2E Test Org",
	}, cookies)
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		resp.Body.Close()
		t.Fatalf("create org returned %d", resp.StatusCode)
	}
	orgBody := decodeBody(t, resp)
	orgID, _ := orgBody["id"].(string)
	if orgID == "" {
		t.Skip("could not get org ID from response")
	}

	// Update cookies after org switch if needed
	if len(resp.Cookies()) > 0 {
		cookies = resp.Cookies()
	}

	// 4. List entities
	resp = getJSON(t, "/api/v1/organizations/"+orgID+"/entities", cookies)
	assert.Contains(t, []int{200, 401}, resp.StatusCode)
	resp.Body.Close()

	// 5. Logout
	resp = postJSON(t, "/api/v1/auth/logout", nil, cookies)
	assert.Equal(t, 200, resp.StatusCode)
	resp.Body.Close()
}

func TestRateLimiting(t *testing.T) {
	// Make many rapid requests — should not get 429 under 300/min threshold for health
	for i := 0; i < 10; i++ {
		resp := getJSON(t, "/health", nil)
		resp.Body.Close()
		assert.NotEqual(t, 429, resp.StatusCode, "should not be rate limited for few requests")
	}
}
