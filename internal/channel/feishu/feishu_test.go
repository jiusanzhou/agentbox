package feishu

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"go.zoe.im/agentbox/internal/channel"
)

func assert(t *testing.T, cond bool, msgs ...string) {
	t.Helper()
	if !cond {
		msg := "assertion failed"
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		t.Fatal(msg)
	}
}

func newTestFeishu(t *testing.T) *Feishu {
	t.Helper()
	f, err := New(Config{
		AppID:             "test-app-id",
		AppSecret:         "test-app-secret",
		VerificationToken: "test-verify-token",
	}, nil)
	assert(t, err == nil, "new feishu failed: "+fmt.Sprint(err))
	f.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})
	return f
}

func TestFeishu_New(t *testing.T) {
	// Valid config
	f, err := New(Config{AppID: "id", AppSecret: "secret"}, nil)
	assert(t, err == nil, "should create feishu")
	assert(t, f.Name() == "feishu", "name should be feishu")
	assert(t, f.Path() == "/api/v1/feishu/callback", "default path")

	// Custom path
	f, err = New(Config{AppID: "id", AppSecret: "secret", CallbackPath: "/custom"}, nil)
	assert(t, err == nil, "should create feishu with custom path")
	assert(t, f.Path() == "/custom", "custom path")

	// Missing config
	_, err = New(Config{}, nil)
	assert(t, err != nil, "should fail without app_id and app_secret")
}

func TestFeishu_URLVerification(t *testing.T) {
	f := newTestFeishu(t)

	body := `{"type":"url_verification","token":"test-verify-token","challenge":"test-challenge-123"}`
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	f.HandleIncoming(w, req)

	assert(t, w.Code == http.StatusOK, fmt.Sprintf("expected 200, got %d", w.Code))

	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	assert(t, err == nil, "decode response failed")
	assert(t, resp["challenge"] == "test-challenge-123", "challenge mismatch")
}

func TestFeishu_URLVerificationBadToken(t *testing.T) {
	f := newTestFeishu(t)

	body := `{"type":"url_verification","token":"wrong-token","challenge":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(body))
	w := httptest.NewRecorder()

	f.HandleIncoming(w, req)
	assert(t, w.Code == http.StatusForbidden, fmt.Sprintf("expected 403, got %d", w.Code))
}

func TestFeishu_MessageEvent(t *testing.T) {
	f, _ := New(Config{
		AppID:             "test-app-id",
		AppSecret:         "test-app-secret",
		VerificationToken: "test-verify-token",
	}, nil)

	var received *channel.Message
	f.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		received = msg
		return nil
	})

	event := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"event_id":   "ev-123",
			"event_type": "im.message.receive_v1",
			"token":      "test-verify-token",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id":   map[string]string{"open_id": "ou_user1"},
				"sender_type": "user",
			},
			"message": map[string]any{
				"message_id":   "msg-001",
				"chat_id":      "oc_chat1",
				"message_type": "text",
				"content":      `{"text":"hello agent"}`,
			},
		},
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	f.HandleIncoming(w, req)

	assert(t, w.Code == http.StatusOK, fmt.Sprintf("expected 200, got %d", w.Code))
	assert(t, received != nil, "handler should have been called")
	assert(t, received.Text == "hello agent", "text mismatch: "+received.Text)
	assert(t, received.ChatID == "oc_chat1", "chatID mismatch")
	assert(t, received.UserID == "ou_user1", "userID mismatch")
	assert(t, received.Extra["channel"] == "feishu", "channel metadata mismatch")
}

func TestFeishu_MessageEventStripMention(t *testing.T) {
	f, _ := New(Config{
		AppID:             "test-app-id",
		AppSecret:         "test-app-secret",
		VerificationToken: "test-verify-token",
	}, nil)

	var received *channel.Message
	f.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		received = msg
		return nil
	})

	event := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"event_id":   "ev-strip",
			"event_type": "im.message.receive_v1",
			"token":      "test-verify-token",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]string{"open_id": "ou_user1"},
			},
			"message": map[string]any{
				"message_id":   "msg-strip",
				"chat_id":      "oc_chat1",
				"message_type": "text",
				"content":      `{"text":"@_user_123 hello bot"}`,
			},
		},
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	f.HandleIncoming(w, req)

	assert(t, received != nil, "handler should be called")
	assert(t, received.Text == "hello bot", "mention should be stripped, got: "+received.Text)
}

func TestFeishu_EventDeduplication(t *testing.T) {
	f, _ := New(Config{
		AppID:             "test-app-id",
		AppSecret:         "test-app-secret",
		VerificationToken: "test-verify-token",
	}, nil)

	var callCount int32
	f.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	})

	event := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"event_id":   "ev-dedup",
			"event_type": "im.message.receive_v1",
			"token":      "test-verify-token",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]string{"open_id": "ou_user1"},
			},
			"message": map[string]any{
				"message_id":   "msg-dedup",
				"chat_id":      "oc_chat1",
				"message_type": "text",
				"content":      `{"text":"hello"}`,
			},
		},
	}

	body, _ := json.Marshal(event)

	// Send the same event twice
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(string(body)))
		w := httptest.NewRecorder()
		f.HandleIncoming(w, req)
		assert(t, w.Code == http.StatusOK, "both requests should return 200")
	}

	assert(t, atomic.LoadInt32(&callCount) == 1, fmt.Sprintf("handler should be called once, got %d", callCount))
}

func TestFeishu_AESDecrypt(t *testing.T) {
	encryptKey := "test-encrypt-key-123"

	f, _ := New(Config{
		AppID:      "test-app-id",
		AppSecret:  "test-app-secret",
		EncryptKey: encryptKey,
	}, nil)

	// Encrypt a known plaintext using the same algorithm
	plaintext := `{"type":"url_verification","challenge":"encrypted-challenge","token":"tok"}`

	key := sha256.Sum256([]byte(encryptKey))
	block, err := aes.NewCipher(key[:])
	assert(t, err == nil, "create cipher failed")

	// PKCS#7 pad
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	// IV = first block
	iv := make([]byte, aes.BlockSize)
	for i := range iv {
		iv[i] = byte(i) // deterministic for testing
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Prepend IV
	full := append(iv, ciphertext...)
	encoded := base64.StdEncoding.EncodeToString(full)

	// Decrypt
	result, err := f.decrypt(encoded)
	assert(t, err == nil, "decrypt failed: "+fmt.Sprint(err))
	assert(t, result == plaintext, "decrypted text mismatch: "+result)
}

func TestFeishu_DecryptBadBase64(t *testing.T) {
	f, _ := New(Config{
		AppID:      "test-app-id",
		AppSecret:  "test-app-secret",
		EncryptKey: "key",
	}, nil)

	_, err := f.decrypt("not-valid-base64!!!")
	assert(t, err != nil, "should fail on bad base64")
}

func TestFeishu_DecryptShortCiphertext(t *testing.T) {
	f, _ := New(Config{
		AppID:      "test-app-id",
		AppSecret:  "test-app-secret",
		EncryptKey: "key",
	}, nil)

	// Too short to contain IV
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err := f.decrypt(short)
	assert(t, err != nil, "should fail on short ciphertext")
}

func TestFeishu_CardActionCallback(t *testing.T) {
	f, _ := New(Config{
		AppID:             "test-app-id",
		AppSecret:         "test-app-secret",
		VerificationToken: "test-verify-token",
	}, nil)

	var receivedCB *channel.Callback
	f.OnCallback(func(ctx context.Context, cb *channel.Callback) error {
		receivedCB = cb
		return nil
	})
	f.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})

	event := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"event_id":   "ev-card",
			"event_type": "card.action.trigger",
			"token":      "test-verify-token",
		},
		"event": map[string]any{
			"operator": map[string]any{
				"open_id": "ou_operator",
			},
			"action": map[string]any{
				"tag":   "button",
				"value": map[string]string{"action": "permission_allow_abc"},
			},
			"context": map[string]any{
				"open_chat_id":    "oc_chat1",
				"open_message_id": "om_msg1",
			},
		},
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	f.HandleIncoming(w, req)

	assert(t, w.Code == http.StatusOK, fmt.Sprintf("expected 200, got %d", w.Code))
	assert(t, receivedCB != nil, "callback handler should have been called")
	assert(t, receivedCB.ID == "permission_allow_abc", "action ID mismatch")
	assert(t, receivedCB.ChatID == "oc_chat1", "chat ID mismatch")
	assert(t, receivedCB.UserID == "ou_operator", "user ID mismatch")
	assert(t, receivedCB.MessageID == "om_msg1", "message ID mismatch")
}

func TestFeishu_TenantAccessTokenCaching(t *testing.T) {
	var callCount int32

	// Mock Feishu API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "tenant_access_token") {
			atomic.AddInt32(&callCount, 1)
			json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "test-token-abc",
				"expire":             7200,
			})
			return
		}
		// Send message API
		if strings.Contains(r.URL.Path, "im/v1/messages") {
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok"})
			return
		}
	}))
	defer apiServer.Close()

	f, _ := New(Config{AppID: "id", AppSecret: "secret"}, nil)
	f.client = apiServer.Client()

	// Override the URLs to point to our test server
	// We can't easily override the hardcoded URLs, so let's test
	// getTenantAccessToken indirectly through Send, using a mock HTTP client
	// that intercepts the requests

	// Instead, let's test the caching logic directly by creating a transport
	f.client = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "tenant_access_token") {
					atomic.AddInt32(&callCount, 1)
					w := httptest.NewRecorder()
					json.NewEncoder(w).Encode(map[string]any{
						"code":                0,
						"msg":                 "ok",
						"tenant_access_token": "cached-token",
						"expire":             7200,
					})
					return w.Result(), nil
				}
				// Messages API
				w := httptest.NewRecorder()
				json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok"})
				return w.Result(), nil
			},
		},
	}

	atomic.StoreInt32(&callCount, 0)

	// First call — should fetch token
	ctx := context.Background()
	err := f.Send(ctx, "chat1", "msg1", nil)
	assert(t, err == nil, "first send failed: "+fmt.Sprint(err))

	// Second call — should use cached token
	err = f.Send(ctx, "chat1", "msg2", nil)
	assert(t, err == nil, "second send failed: "+fmt.Sprint(err))

	count := atomic.LoadInt32(&callCount)
	assert(t, count == 1, fmt.Sprintf("token should be fetched once (cached), got %d calls", count))
}

func TestFeishu_SendMessage(t *testing.T) {
	f, _ := New(Config{AppID: "id", AppSecret: "secret"}, nil)

	var sentPayload map[string]string
	f.client = &http.Client{
		Transport: &testTransport{
			handler: func(req *http.Request) (*http.Response, error) {
				if strings.Contains(req.URL.Path, "tenant_access_token") {
					w := httptest.NewRecorder()
					json.NewEncoder(w).Encode(map[string]any{
						"code":                0,
						"tenant_access_token": "tok",
						"expire":             7200,
					})
					return w.Result(), nil
				}
				// Capture the sent message
				json.NewDecoder(req.Body).Decode(&sentPayload)
				w := httptest.NewRecorder()
				json.NewEncoder(w).Encode(map[string]any{"code": 0})
				return w.Result(), nil
			},
		},
	}

	err := f.Send(context.Background(), "chat-send", "hello world", nil)
	assert(t, err == nil, "send failed: "+fmt.Sprint(err))
	assert(t, sentPayload["receive_id"] == "chat-send", "receive_id mismatch")
	assert(t, sentPayload["msg_type"] == "text", "msg_type should be text")
}

func TestFeishu_InvalidJSON(t *testing.T) {
	f := newTestFeishu(t)

	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	f.HandleIncoming(w, req)
	assert(t, w.Code == http.StatusBadRequest, "should return 400 for invalid json")
}

func TestFeishu_NonTextMessage(t *testing.T) {
	f, _ := New(Config{
		AppID:             "test-app-id",
		AppSecret:         "test-app-secret",
		VerificationToken: "test-verify-token",
	}, nil)

	var called bool
	f.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		called = true
		return nil
	})

	event := map[string]any{
		"schema": "2.0",
		"header": map[string]any{
			"event_id":   "ev-img",
			"event_type": "im.message.receive_v1",
			"token":      "test-verify-token",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]string{"open_id": "ou_user1"},
			},
			"message": map[string]any{
				"message_id":   "msg-img",
				"chat_id":      "oc_chat1",
				"message_type": "image",
				"content":      `{"image_key":"key123"}`,
			},
		},
	}

	body, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	f.HandleIncoming(w, req)

	assert(t, !called, "handler should not be called for non-text messages")
}

func TestFeishu_EncryptedEvent(t *testing.T) {
	encryptKey := "my-secret-key"
	f, _ := New(Config{
		AppID:             "test-app-id",
		AppSecret:         "test-app-secret",
		VerificationToken: "tok",
		EncryptKey:        encryptKey,
	}, nil)
	f.Start(context.Background(), func(ctx context.Context, msg *channel.Message) error {
		return nil
	})

	// Build a URL verification challenge payload
	inner := `{"type":"url_verification","challenge":"enc-challenge","token":"tok"}`

	// Encrypt it
	key := sha256.Sum256([]byte(encryptKey))
	block, _ := aes.NewCipher(key[:])
	padLen := aes.BlockSize - (len(inner) % aes.BlockSize)
	padded := make([]byte, len(inner)+padLen)
	copy(padded, inner)
	for i := len(inner); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}
	iv := make([]byte, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	encoded := base64.StdEncoding.EncodeToString(append(iv, ciphertext...))

	outer, _ := json.Marshal(map[string]string{"encrypt": encoded})
	req := httptest.NewRequest(http.MethodPost, "/callback", strings.NewReader(string(outer)))
	w := httptest.NewRecorder()
	f.HandleIncoming(w, req)

	assert(t, w.Code == http.StatusOK, fmt.Sprintf("expected 200, got %d", w.Code))
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	assert(t, resp["challenge"] == "enc-challenge", "decrypted challenge mismatch")
}

func TestFeishu_Stop(t *testing.T) {
	f := newTestFeishu(t)
	err := f.Stop(context.Background())
	assert(t, err == nil, "stop should not error")
}

// testTransport is a simple http.RoundTripper for mocking HTTP requests.
type testTransport struct {
	handler func(req *http.Request) (*http.Response, error)
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.handler(req)
}
