package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"go.zoe.im/agentbox/internal/dna"
	"go.zoe.im/agentbox/internal/model"

	"go.zoe.im/x/cli"
)

var (
	publishToken string
)

var publishCmd = cli.New(
	cli.Name("publish"),
	cli.Short("Validate and publish an agent DNA directory to the registry"),
	cli.Run(runPublish),
)

func init() {
	publishCmd.Flags().StringVarP(&publishToken, "token", "t", "", "Auth token or API key (default: AGENTBOX_TOKEN env)")
}

func runPublish(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl publish <agent-dna-dir>")
		os.Exit(1)
	}

	dir := args[0]
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid path %s: %v\n", dir, err)
		os.Exit(1)
	}

	// Parse DNA directory
	fmt.Printf("Parsing agent DNA from %s ...\n", absDir)
	agent, err := dna.ParseDir(absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Validate
	fmt.Println("Validating ...")
	if err := dna.Validate(agent); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Name:    %s\n", agent.Identity.Name)
	fmt.Printf("  Slug:    %s\n", agent.Slug)
	fmt.Printf("  Version: %s\n", agent.Version)
	fmt.Printf("  Runtime: %s\n", agent.Manifest.Runtime)

	// Resolve auth token
	token := publishToken
	if token == "" {
		token = os.Getenv("AGENTBOX_TOKEN")
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: --token or AGENTBOX_TOKEN is required")
		os.Exit(1)
	}

	// Resolve server address
	addr := serverAddr
	if env := os.Getenv("AGENTBOX_SERVER"); env != "" {
		addr = env
	}

	// Build publish request
	reqBody := struct {
		Slug     string               `json:"slug"`
		Identity *model.AgentIdentity `json:"identity"`
		Soul     *model.AgentSoul     `json:"soul,omitempty"`
		Tools    json.RawMessage      `json:"tools,omitempty"`
		Memory   json.RawMessage      `json:"memory,omitempty"`
		Skills   json.RawMessage      `json:"skills,omitempty"`
		Manifest *model.AgentManifest `json:"manifest"`
		RepoURL  string               `json:"repo_url,omitempty"`
		RepoRef  string               `json:"repo_ref,omitempty"`
	}{
		Slug:     agent.Slug,
		Identity: agent.Identity,
		Soul:     agent.Soul,
		Tools:    agent.Tools,
		Memory:   agent.Memory,
		Skills:   agent.Skills,
		Manifest: agent.Manifest,
	}

	// Try to detect git repo info
	if repoURL, ref := detectGitInfo(absDir); repoURL != "" {
		reqBody.RepoURL = repoURL
		reqBody.RepoRef = ref
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: marshal: %v\n", err)
		os.Exit(1)
	}

	// POST to publish endpoint
	fmt.Printf("Publishing to %s ...\n", addr)
	url := addr + "/api/v1/registry/agents/publish"
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "Error (%d): %s\n", resp.StatusCode, string(respBody))
		os.Exit(1)
	}

	// Parse response to show version
	var result model.AgentDNA
	if err := json.Unmarshal(respBody, &result); err == nil {
		fmt.Printf("\nPublished %s v%s\n", result.Slug, result.Version)
		fmt.Printf("  ID:     %s\n", result.ID)
		fmt.Printf("  Status: %s\n", result.Status)
	} else {
		fmt.Printf("\nPublished successfully.\n%s\n", string(respBody))
	}
}

// detectGitInfo tries to read git remote URL and HEAD ref from a directory.
func detectGitInfo(dir string) (repoURL, ref string) {
	// Walk up to find .git
	d := dir
	for {
		if info, err := os.Stat(filepath.Join(d, ".git")); err == nil && info.IsDir() {
			// Read HEAD
			if headData, err := os.ReadFile(filepath.Join(d, ".git", "HEAD")); err == nil {
				ref = string(bytes.TrimSpace(headData))
			}
			// Read origin URL from config (simple parse)
			if configData, err := os.ReadFile(filepath.Join(d, ".git", "config")); err == nil {
				repoURL = parseGitConfigURL(string(configData))
			}
			return
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return
}

// parseGitConfigURL extracts the origin remote URL from git config content.
func parseGitConfigURL(config string) string {
	inOrigin := false
	for _, line := range bytes.Split([]byte(config), []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if bytes.Equal(trimmed, []byte(`[remote "origin"]`)) {
			inOrigin = true
			continue
		}
		if len(trimmed) > 0 && trimmed[0] == '[' {
			inOrigin = false
			continue
		}
		if inOrigin && bytes.HasPrefix(trimmed, []byte("url = ")) {
			return string(bytes.TrimPrefix(trimmed, []byte("url = ")))
		}
	}
	return ""
}
