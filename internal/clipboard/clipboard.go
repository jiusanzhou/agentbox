package clipboard

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

type Clipboard struct{}

func New() *Clipboard { return &Clipboard{} }

func (c *Clipboard) Handler() http.Handler { return c }

func (c *Clipboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodGet && (path == "" || path == "text"):
		c.handleRead(w, r)
	case r.Method == http.MethodPost && (path == "" || path == "text"):
		c.handleWrite(w, r)
	case r.Method == http.MethodGet && path == "image":
		c.handleImage(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (c *Clipboard) handleRead(w http.ResponseWriter, r *http.Request) {
	text, err := readClipboard()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": text})
}

func (c *Clipboard) handleWrite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if err := writeClipboard(req.Text); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (c *Clipboard) handleImage(w http.ResponseWriter, r *http.Request) {
	data, err := readClipboardImage()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"image": ""})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"image": base64.StdEncoding.EncodeToString(data),
	})
}

func readClipboard() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("pbpaste").Output()
		return string(out), err
	default: // linux
		out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
		if err != nil {
			out, err = exec.Command("xsel", "--clipboard", "--output").Output()
		}
		return string(out), err
	}
}

func writeClipboard(text string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	default: // linux
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		return nil
	}
}

func readClipboardImage() ([]byte, error) {
	switch runtime.GOOS {
	case "darwin":
		// Use osascript to check if clipboard has image, then use pngpaste if available
		out, err := exec.Command("osascript", "-e",
			`the clipboard as «class PNGf»`).Output()
		if err != nil {
			return nil, err
		}
		return out, nil
	default:
		var buf bytes.Buffer
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-o")
		cmd.Stdout = &buf
		if err := cmd.Run(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
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
