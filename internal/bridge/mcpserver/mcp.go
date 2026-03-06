package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Server struct {
	roots  []string
	logger *slog.Logger
}

func New(roots []string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{roots: roots, logger: logger}
}

// JSON-RPC types
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

// MCP types
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// RunStdio runs the MCP server on stdin/stdout.
func (s *Server) RunStdio() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		resp := s.handle(req)
		if resp != nil {
			data, _ := json.Marshal(resp)
			fmt.Fprintln(writer, string(data))
			writer.Flush()
		}
	}
	return scanner.Err()
}

func (s *Server) handle(req jsonrpcRequest) *jsonrpcResponse {
	switch req.Method {
	case "initialize":
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "abox-bridge",
					"version": "1.0.0",
				},
			},
		}

	case "notifications/initialized":
		return nil // no response

	case "tools/list":
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": s.listTools(),
			},
		}

	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		json.Unmarshal(req.Params, &params)
		result := s.callTool(params.Name, params.Arguments)
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		return &jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: map[string]any{
				"code":    -32601,
				"message": "method not found: " + req.Method,
			},
		}
	}
}

func (s *Server) listTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "list_directory",
			Description: "List files and directories in a path. Returns names with [DIR] or [FILE] prefix.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "Directory path relative to allowed roots"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "read_file",
			Description: "Read the contents of a file. Supports text files.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":   map[string]any{"type": "string", "description": "File path relative to allowed roots"},
					"offset": map[string]any{"type": "integer", "description": "Start line (0-based)"},
					"limit":  map[string]any{"type": "integer", "description": "Max lines to read"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "search_files",
			Description: "Search for files by name pattern (glob). Returns matching file paths.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{"type": "string", "description": "Glob pattern (e.g. *.go, **/*.md)"},
					"path":    map[string]any{"type": "string", "description": "Directory to search in"},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "grep",
			Description: "Search file contents for a text pattern. Returns matching lines with file paths.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Text to search for"},
					"path":  map[string]any{"type": "string", "description": "Directory to search in"},
					"glob":  map[string]any{"type": "string", "description": "File pattern filter (e.g. *.go)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "file_info",
			Description: "Get file metadata: size, modification time, permissions.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string", "description": "File path"},
				},
				"required": []string{"path"},
			},
		},
	}
}

func (s *Server) callTool(name string, args map[string]any) *mcpToolResult {
	switch name {
	case "list_directory":
		return s.toolListDir(strArg(args, "path"))
	case "read_file":
		return s.toolReadFile(strArg(args, "path"), intArg(args, "offset"), intArg(args, "limit"))
	case "search_files":
		return s.toolSearchFiles(strArg(args, "pattern"), strArg(args, "path"))
	case "grep":
		return s.toolGrep(strArg(args, "query"), strArg(args, "path"), strArg(args, "glob"))
	case "file_info":
		return s.toolFileInfo(strArg(args, "path"))
	default:
		return &mcpToolResult{
			Content: []mcpContent{{Type: "text", Text: "unknown tool: " + name}},
			IsError: true,
		}
	}
}

func (s *Server) resolvePath(path string) (string, error) {
	// Try each root
	for _, root := range s.roots {
		full := filepath.Join(root, path)
		abs, err := filepath.Abs(full)
		if err != nil {
			continue
		}
		rootAbs, _ := filepath.Abs(root)
		if !strings.HasPrefix(abs, rootAbs) {
			continue // path escape
		}
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}
	// Try as absolute path, check if within roots
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %s", path)
	}
	for _, root := range s.roots {
		rootAbs, _ := filepath.Abs(root)
		if strings.HasPrefix(abs, rootAbs) {
			if _, err := os.Stat(abs); err == nil {
				return abs, nil
			}
		}
	}
	return "", fmt.Errorf("path not within allowed roots: %s", path)
}

func (s *Server) toolListDir(path string) *mcpToolResult {
	if path == "" || path == "." {
		// List all roots
		var lines []string
		for _, r := range s.roots {
			lines = append(lines, fmt.Sprintf("[ROOT] %s", r))
		}
		return textResult(strings.Join(lines, "\n"))
	}
	resolved, err := s.resolvePath(path)
	if err != nil {
		return errResult(err.Error())
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return errResult(err.Error())
	}
	var lines []string
	for _, e := range entries {
		prefix := "[FILE]"
		if e.IsDir() {
			prefix = "[DIR] "
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, e.Name()))
	}
	return textResult(strings.Join(lines, "\n"))
}

func (s *Server) toolReadFile(path string, offset, limit int) *mcpToolResult {
	resolved, err := s.resolvePath(path)
	if err != nil {
		return errResult(err.Error())
	}
	f, err := os.Open(resolved)
	if err != nil {
		return errResult(err.Error())
	}
	defer f.Close()

	if limit <= 0 {
		limit = 500
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var lines []string
	lineNum := 0
	for scanner.Scan() {
		if lineNum >= offset {
			lines = append(lines, scanner.Text())
			if len(lines) >= limit {
				break
			}
		}
		lineNum++
	}
	return textResult(strings.Join(lines, "\n"))
}

func (s *Server) toolSearchFiles(pattern, path string) *mcpToolResult {
	var searchRoots []string
	if path != "" {
		resolved, err := s.resolvePath(path)
		if err != nil {
			return errResult(err.Error())
		}
		searchRoots = []string{resolved}
	} else {
		searchRoots = s.roots
	}

	var matches []string
	for _, root := range searchRoots {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == ".next" || name == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}
			matched, _ := filepath.Match(pattern, d.Name())
			if matched {
				matches = append(matches, p)
			}
			if len(matches) >= 100 {
				return io.EOF
			}
			return nil
		})
	}
	return textResult(strings.Join(matches, "\n"))
}

func (s *Server) toolGrep(query, path, glob string) *mcpToolResult {
	var searchRoots []string
	if path != "" {
		resolved, err := s.resolvePath(path)
		if err != nil {
			return errResult(err.Error())
		}
		searchRoots = []string{resolved}
	} else {
		searchRoots = s.roots
	}

	var results []string
	for _, root := range searchRoots {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == ".next" || name == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}
			if glob != "" {
				matched, _ := filepath.Match(glob, d.Name())
				if !matched {
					return nil
				}
			}
			// Skip binary files
			info, _ := d.Info()
			if info != nil && info.Size() > 5*1024*1024 {
				return nil
			}
			f, err := os.Open(p)
			if err != nil {
				return nil
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			lineNum := 1
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), query) {
					results = append(results, fmt.Sprintf("%s:%d: %s", p, lineNum, scanner.Text()))
					if len(results) >= 50 {
						return io.EOF
					}
				}
				lineNum++
			}
			return nil
		})
	}
	if len(results) == 0 {
		return textResult("no matches found")
	}
	return textResult(strings.Join(results, "\n"))
}

func (s *Server) toolFileInfo(path string) *mcpToolResult {
	resolved, err := s.resolvePath(path)
	if err != nil {
		return errResult(err.Error())
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return errResult(err.Error())
	}
	text := fmt.Sprintf("Path: %s\nSize: %d bytes\nModified: %s\nIsDir: %v\nMode: %s",
		resolved, info.Size(), info.ModTime().Format("2006-01-02 15:04:05"), info.IsDir(), info.Mode())
	return textResult(text)
}

func textResult(text string) *mcpToolResult {
	return &mcpToolResult{Content: []mcpContent{{Type: "text", Text: text}}}
}

func errResult(text string) *mcpToolResult {
	return &mcpToolResult{Content: []mcpContent{{Type: "text", Text: text}}, IsError: true}
}

func strArg(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func intArg(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}
