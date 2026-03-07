package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Notify struct{}

func New() *Notify { return &Notify{} }

func (n *Notify) Handler() http.Handler { return n }

func (n *Notify) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodPost && path == "send":
		n.handleSend(w, r)
	case r.Method == http.MethodPost && path == "ask":
		n.handleAsk(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (n *Notify) handleSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Sound bool   `json:"sound"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body is required"})
		return
	}
	if req.Title == "" {
		req.Title = "ABox"
	}

	if err := sendNotification(req.Title, req.Body, req.Sound); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (n *Notify) handleAsk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title   string   `json:"title"`
		Body    string   `json:"body"`
		Buttons []string `json:"buttons"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body is required"})
		return
	}
	if len(req.Buttons) == 0 {
		req.Buttons = []string{"OK", "Cancel"}
	}
	if req.Title == "" {
		req.Title = "ABox"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	answer, err := showDialog(ctx, req.Title, req.Body, req.Buttons)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"answer": answer})
}

func sendNotification(title, body string, sound bool) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		if sound {
			script += ` sound name "default"`
		}
		return exec.Command("osascript", "-e", script).Run()
	default: // linux
		args := []string{title, body}
		return exec.Command("notify-send", args...).Run()
	}
}

func showDialog(ctx context.Context, title, body string, buttons []string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		btnList := `"` + strings.Join(buttons, `", "`) + `"`
		script := fmt.Sprintf(
			`display dialog %q with title %q buttons {%s} default button 1`,
			body, title, btnList,
		)
		cmd := exec.CommandContext(ctx, "osascript", "-e", script)
		out, err := cmd.Output()
		if err != nil {
			// User pressed cancel or timeout
			if ctx.Err() != nil {
				return "", fmt.Errorf("dialog timed out")
			}
			return "", err
		}
		// osascript returns: "button returned:Allow"
		result := strings.TrimSpace(string(out))
		if strings.HasPrefix(result, "button returned:") {
			return strings.TrimPrefix(result, "button returned:"), nil
		}
		return result, nil
	default: // linux
		args := []string{"--question", "--title", title, "--text", body}
		cmd := exec.CommandContext(ctx, "zenity", args...)
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return "", fmt.Errorf("dialog timed out")
			}
			// zenity returns exit 1 for "No"/"Cancel"
			if len(buttons) > 1 {
				return buttons[1], nil
			}
			return "Cancel", nil
		}
		return buttons[0], nil
	}
}

// --- Helpers ---

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return false
	}
	if len(body) == 0 {
		return true
	}
	if err := json.Unmarshal(body, v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
