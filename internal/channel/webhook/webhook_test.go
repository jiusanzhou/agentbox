package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.zoe.im/agentbox/internal/channel"
)

func assert(t *testing.T, condition bool, msgs ...string) {
	t.Helper()
	if !condition {
		msg := "assertion failed"
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		t.Fatal(msg)
	}
}

func computeHMAC(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestWebhook_New(t *testing.T) {
	w, err := New(Config{Secret: "test-secret"})
	assert(t, err == nil, "new webhook should succeed")
	assert(t, w.Name() == "webhook", "name should be webhook")
	assert(t, w.Path() == "/api/v1/webhook", "default path should be /api/v1/webhook")
}

func TestWebhook_NewCustomPath(t *testing.T) {
	w, err := New(Config{Path: "/custom/hook"})
	assert(t, err == nil, "new webhook should succeed")
	assert(t, w.Path() == "/custom/hook", "custom path should be /custom/hook")
}

func TestWebhook_Start(t *testing.T) {
	w, _ := New(Config{})
	err := w.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})
	assert(t, err == nil, "start should succeed")
	assert(t, w.handler != nil, "handler should be set")
}

func TestWebhook_Stop(t *testing.T) {
	w, _ := New(Config{})
	err := w.Stop(context.Background())
	assert(t, err == nil, "stop should succeed")
}

func TestWebhook_HandleIncoming_WithSignature(t *testing.T) {
	w, _ := New(Config{Secret: "test-secret", ResponseURL: "http://example.com/callback"})

	var received *channel.Message
	w.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		received = msg
		return nil
	})

	body := `{"chat_id":"c1","user_id":"u1","username":"alice","text":"hello"}`
	sig := computeHMAC(body, "test-secret")

	req := httptest.NewRequest("POST", "/hook/test", strings.NewReader(body))
	req.Header.Set("X-Signature", sig)
	rr := httptest.NewRecorder()

	w.HandleIncoming(rr, req)

	assert(t, rr.Code == http.StatusAccepted, "should return 202 for async mode")

	// Give goroutine time to process
	for i := 0; i < 10; i++ {
		if received != nil {
			break
		}
		// Small busy wait
	}
}

func TestWebhook_HandleIncoming_InvalidSignature(t *testing.T) {
	w, _ := New(Config{Secret: "test-secret"})
	w.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})

	body := `{"chat_id":"c1","text":"hello"}`
	req := httptest.NewRequest("POST", "/hook/test", strings.NewReader(body))
	req.Header.Set("X-Signature", "invalid-signature")
	rr := httptest.NewRecorder()

	w.HandleIncoming(rr, req)

	assert(t, rr.Code == http.StatusForbidden, "invalid signature should return 403")
}

func TestWebhook_HandleIncoming_NoSignatureRequired(t *testing.T) {
	w, _ := New(Config{ResponseURL: "http://example.com/callback"})

	w.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})

	body := `{"chat_id":"c1","text":"no sig needed"}`
	req := httptest.NewRequest("POST", "/hook/test", strings.NewReader(body))
	rr := httptest.NewRecorder()

	w.HandleIncoming(rr, req)

	assert(t, rr.Code == http.StatusAccepted, "should accept without signature when no secret")
}

func TestWebhook_HandleIncoming_InvalidJSON(t *testing.T) {
	w, _ := New(Config{})
	w.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})

	req := httptest.NewRequest("POST", "/hook/test", strings.NewReader("not json"))
	rr := httptest.NewRecorder()

	w.HandleIncoming(rr, req)

	assert(t, rr.Code == http.StatusBadRequest, "invalid json should return 400")
}

func TestWebhook_HandleIncoming_EmptyText(t *testing.T) {
	w, _ := New(Config{})
	w.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})

	body := `{"chat_id":"c1","text":""}`
	req := httptest.NewRequest("POST", "/hook/test", strings.NewReader(body))
	rr := httptest.NewRecorder()

	w.HandleIncoming(rr, req)

	assert(t, rr.Code == http.StatusBadRequest, "empty text should return 400")
}

func TestWebhook_HandleIncoming_DefaultChatID(t *testing.T) {
	w, _ := New(Config{ResponseURL: "http://example.com/callback"})

	w.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})

	body := `{"text":"no chat id"}`
	req := httptest.NewRequest("POST", "/hook/test", strings.NewReader(body))
	rr := httptest.NewRecorder()

	w.HandleIncoming(rr, req)

	assert(t, rr.Code == http.StatusAccepted, "should accept without chat_id")
}

func TestWebhook_Send_AsyncCallback(t *testing.T) {
	// Set up a test server to receive the callback
	var callbackReceived bool
	var callbackBody map[string]string
	callbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callbackReceived = true
		json.NewDecoder(r.Body).Decode(&callbackBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer callbackServer.Close()

	w, _ := New(Config{ResponseURL: callbackServer.URL})
	w.Start(context.Background(), nil)

	err := w.Send(context.Background(), "c1", "hello response", nil)
	assert(t, err == nil, "send should succeed")
	assert(t, callbackReceived, "callback should be received")
	assert(t, callbackBody["text"] == "hello response", "callback text should match")
	assert(t, callbackBody["chat_id"] == "c1", "callback chat_id should match")
}

func TestWebhook_VerifyHMAC(t *testing.T) {
	w, _ := New(Config{Secret: "my-secret"})

	body := []byte("test body")
	mac := hmac.New(sha256.New, []byte("my-secret"))
	mac.Write(body)
	validSig := hex.EncodeToString(mac.Sum(nil))

	assert(t, w.verifyHMAC(body, validSig), "valid HMAC should pass")
	assert(t, !w.verifyHMAC(body, "invalid"), "invalid HMAC should fail")
	assert(t, !w.verifyHMAC([]byte("different body"), validSig), "different body should fail")
}
