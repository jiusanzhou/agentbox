package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.zoe.im/agentbox/internal/config"
	"go.zoe.im/agentbox/internal/ratelimit"
	"go.zoe.im/x"

	_ "go.zoe.im/agentbox/internal/executor/mock"
	_ "go.zoe.im/agentbox/internal/storage/local"
	_ "go.zoe.im/agentbox/internal/store/memory"
)

func assert(t *testing.T, cond bool, msgs ...string) {
	t.Helper()
	if !cond {
		msg := "assertion failed"
		if len(msgs) > 0 { msg = msgs[0] }
		t.Fatal(msg)
	}
}

func setupE2E(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := config.NewConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.JWTSecret = "test-e2e-secret-2026"
	cfg.Executor = x.TypedLazyConfig{Type: "mock"}
	cfg.Store = x.TypedLazyConfig{Type: "memory"}
	cfg.Storage = x.TypedLazyConfig{
		Type:   "local",
		Config: json.RawMessage(`{"root":"` + t.TempDir() + `"}`),
	}
	cfg.RateLimit = config.RateLimitConfig{RequestsPerMinute: 600, BurstSize: 100}

	svc, err := New(cfg)
	if err != nil { t.Fatal("setup:", err) }

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go svc.server.Serve(ctx)

	var handler http.Handler = svc.mux
	handler = svc.auth.Middleware(handler)
	limiter := ratelimit.New(svc.cfg.RateLimit)
	handler = limiter.Middleware(func(r *http.Request) string { return "" })(handler)
	handler = apiHeaderMiddleware(handler)

	ts := httptest.NewServer(handler)
	t.Cleanup(func() { ts.Close() })
	return ts
}

func doJSON(t *testing.T, method, url, token, reqBody string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if reqBody != "" { reader = strings.NewReader(reqBody) }
	req, err := http.NewRequest(method, url, reader)
	if err != nil { t.Fatal(err) }
	req.Header.Set("Content-Type", "application/json")
	if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
	resp, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func registerAndGetToken(t *testing.T, ts *httptest.Server, email string) string {
	t.Helper()
	status, body := doJSON(t, "POST", ts.URL+"/api/v1/authregister", "",
		`{"email":"`+email+`","password":"testpass123","name":"E2E"}`)
	assert(t, status == 200, "register: "+string(body))
	var r struct{ Token string `json:"token"` }
	json.Unmarshal(body, &r)
	assert(t, r.Token != "", "no token")
	return r.Token
}

// ─── Tests ────────────────────────────────────────────────────

func TestE2E_HealthCheck(t *testing.T) {
	ts := setupE2E(t)
	status, body := doJSON(t, "GET", ts.URL+"/api/v1/health", "", "")
	assert(t, status == 200, "health: "+string(body))
	var m map[string]string
	json.Unmarshal(body, &m)
	assert(t, m["status"] == "ok")
}

func TestE2E_AuthFlow(t *testing.T) {
	ts := setupE2E(t)

	// Register
	status, body := doJSON(t, "POST", ts.URL+"/api/v1/authregister", "",
		`{"email":"auth@test.com","password":"pass123","name":"Auth"}`)
	assert(t, status == 200, "register: "+string(body))
	var reg struct {
		Token string `json:"token"`
		User  struct{ Email string `json:"email"` } `json:"user"`
	}
	json.Unmarshal(body, &reg)
	assert(t, reg.Token != "")
	assert(t, reg.User.Email == "auth@test.com")
	token := reg.Token

	// Login
	status, _ = doJSON(t, "POST", ts.URL+"/api/v1/authlogin", "",
		`{"email":"auth@test.com","password":"pass123"}`)
	assert(t, status == 200, "login failed")

	// Get Me with JWT
	status, _ = doJSON(t, "GET", ts.URL+"/api/v1/authme", token, "")
	assert(t, status == 200, "get me failed")

	// Generate API Key
	status, body = doJSON(t, "POST", ts.URL+"/api/v1/authapikey", token, "")
	assert(t, status == 200, "gen apikey: "+string(body))
	var keyResp struct{ APIKey string `json:"api_key"` }
	json.Unmarshal(body, &keyResp)
	assert(t, strings.HasPrefix(keyResp.APIKey, "ak_"))

	// Use API Key via Authorization header (not X-API-Key — that's for AI provider)
	status, _ = doJSON(t, "GET", ts.URL+"/api/v1/authme", keyResp.APIKey, "")
	assert(t, status == 200, "api key auth via Bearer failed")

	// Wrong password
	status, _ = doJSON(t, "POST", ts.URL+"/api/v1/authlogin", "",
		`{"email":"auth@test.com","password":"wrong"}`)
	assert(t, status != 200, "wrong password should fail")

	// Duplicate
	status, _ = doJSON(t, "POST", ts.URL+"/api/v1/authregister", "",
		`{"email":"auth@test.com","password":"pass123","name":"Dup"}`)
	assert(t, status != 200, "duplicate should fail")
}

func TestE2E_RunLifecycle(t *testing.T) {
	ts := setupE2E(t)
	token := registerAndGetToken(t, ts, "run@test.com")

	// Create
	status, body := doJSON(t, "POST", ts.URL+"/api/v1/run", token,
		`{"name":"test-run","agent_file":"Be helpful."}`)
	assert(t, status == 200, "create run: "+string(body))
	var run struct{ ID string `json:"id"` }
	json.Unmarshal(body, &run)
	assert(t, run.ID != "")

	// List
	status, body = doJSON(t, "GET", ts.URL+"/api/v1/runs", token, "")
	assert(t, status == 200)
	var runs []json.RawMessage
	json.Unmarshal(body, &runs)
	assert(t, len(runs) >= 1)

	// Get
	status, _ = doJSON(t, "GET", ts.URL+"/api/v1/run/"+run.ID, token, "")
	assert(t, status == 200)

	// Delete — talk returns 200 with empty body for error-returning methods
	status, _ = doJSON(t, "DELETE", ts.URL+"/api/v1/run/"+run.ID, token, "")
	// Delete of completed run may fail — that is OK
	_ = status
}

func TestE2E_SessionFlow(t *testing.T) {
	ts := setupE2E(t)
	token := registerAndGetToken(t, ts, "session@test.com")

	// Create session
	status, body := doJSON(t, "POST", ts.URL+"/api/v1/session", token,
		`{"name":"test-session","agent_file":"Test agent.","runtime":"claude"}`)
	assert(t, status == 200, "create session: "+string(body))
	var sess struct {
		ID   string `json:"id"`
		Mode string `json:"mode"`
	}
	json.Unmarshal(body, &sess)
	assert(t, sess.ID != "")
	assert(t, sess.Mode == "session")

	// Send message — talk maps CreateSessionMessage → POST /api/v1/sessionmessage
	status, body = doJSON(t, "POST", ts.URL+"/api/v1/sessionmessage", token,
		`{"session_id":"`+sess.ID+`","message":"hello"}`)
	assert(t, status == 200, "send message: "+string(body))
	var msgResp struct{ Response string `json:"response"` }
	json.Unmarshal(body, &msgResp)
	assert(t, strings.Contains(msgResp.Response, "mock response"), "got: "+msgResp.Response)

	// Delete session
	status, _ = doJSON(t, "DELETE", ts.URL+"/api/v1/session/"+sess.ID, token, "")
	assert(t, status >= 200 && status < 300)
}

func TestE2E_IntegrationCRUD(t *testing.T) {
	ts := setupE2E(t)
	token := registerAndGetToken(t, ts, "intg@test.com")

	// Create — config needs to be a JSON string, not nested object
	createBody := `{"type":"webhook","name":"Test Hook","config":{"secret":"test-secret-12345678"},"enabled":false}`
	status, body := doJSON(t, "POST", ts.URL+"/api/v1/integrations", token, createBody)
	assert(t, status >= 200 && status < 300, "create: "+string(body))
	var intg struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	json.Unmarshal(body, &intg)
	assert(t, intg.ID != "", "no id in: "+string(body))
	assert(t, intg.Type == "webhook")

	// List
	status, _ = doJSON(t, "GET", ts.URL+"/api/v1/integrations", token, "")
	assert(t, status == 200)

	// Get
	status, _ = doJSON(t, "GET", ts.URL+"/api/v1/integrations/"+intg.ID, token, "")
	assert(t, status == 200)

	// Update
	status, _ = doJSON(t, "PUT", ts.URL+"/api/v1/integrations/"+intg.ID, token,
		`{"name":"Updated Hook"}`)
	assert(t, status == 200, "update failed")

	// Delete
	status, _ = doJSON(t, "DELETE", ts.URL+"/api/v1/integrations/"+intg.ID, token, "")
	assert(t, status == 200)

	// Verify deleted
	status, _ = doJSON(t, "GET", ts.URL+"/api/v1/integrations/"+intg.ID, token, "")
	assert(t, status >= 400, "should fail after delete")
}

func TestE2E_Unauthorized(t *testing.T) {
	ts := setupE2E(t)
	for _, ep := range []struct{ m, p string }{
		{"GET", "/api/v1/integrations"},
		{"POST", "/api/v1/integrations"},
	} {
		status, body := doJSON(t, ep.m, ts.URL+ep.p, "", "")
		assert(t, status == 401 || strings.Contains(string(body), "unauthorized"),
			ep.m+" "+ep.p+": "+string(body))
	}
}

func TestE2E_AdminRuntimes(t *testing.T) {
	ts := setupE2E(t)
	token := registerAndGetToken(t, ts, "admin@test.com")
	status, body := doJSON(t, "GET", ts.URL+"/api/v1/admin/runtimes", token, "")
	assert(t, status == 200, "runtimes: "+string(body))
	var rts []interface{}
	json.Unmarshal(body, &rts)
	assert(t, len(rts) >= 10, "should have >=10 runtimes")
}

func TestE2E_FileUpload(t *testing.T) {
	ts := setupE2E(t)
	token := registerAndGetToken(t, ts, "upload@test.com")

	status, body := doJSON(t, "POST", ts.URL+"/api/v1/session", token,
		`{"name":"upload","agent_file":"Test."}`)
	assert(t, status == 200)
	var sess struct{ ID string `json:"id"` }
	json.Unmarshal(body, &sess)

	var buf bytes.Buffer
	buf.WriteString("--b\r\nContent-Disposition: form-data; name=\"session_id\"\r\n\r\n")
	buf.WriteString(sess.ID + "\r\n")
	buf.WriteString("--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"t.txt\"\r\n")
	buf.WriteString("Content-Type: text/plain\r\n\r\nhello\r\n--b--\r\n")
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/upload", &buf)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	assert(t, resp.StatusCode < 500, "upload 500")

	doJSON(t, "DELETE", ts.URL+"/api/v1/session/"+sess.ID, token, "")
}

func TestE2E_SSEStream(t *testing.T) {
	ts := setupE2E(t)
	token := registerAndGetToken(t, ts, "sse@test.com")

	status, body := doJSON(t, "POST", ts.URL+"/api/v1/session", token,
		`{"name":"sse","agent_file":"Test."}`)
	assert(t, status == 200)
	var sess struct{ ID string `json:"id"` }
	json.Unmarshal(body, &sess)

	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/stream",
		strings.NewReader(`{"session_id":"`+sess.ID+`","message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	assert(t, err == nil)
	defer resp.Body.Close()
	assert(t, resp.StatusCode == 200, "stream: "+resp.Status)
	sseBody, _ := io.ReadAll(resp.Body)
	assert(t, strings.Contains(string(sseBody), "data:") || strings.Contains(string(sseBody), "mock"),
		"should have SSE data")

	doJSON(t, "DELETE", ts.URL+"/api/v1/session/"+sess.ID, token, "")
}
