package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/pkg/agent"
	"go.zoe.im/x/cli"
)

var (
	agentRegistry string
	agentDir      string
)

var agentCmd = cli.New(
	cli.Name("agent"),
	cli.Short("Manage agents from registries"),
)

func init() {
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, ".abox", "agents")

	agentCmd.PersistentFlags().StringVar(&agentRegistry, "registry", "openagent-spec/registry", "GitHub registry (owner/repo)")
	agentCmd.PersistentFlags().StringVar(&agentDir, "dir", defaultDir, "Local agent store directory")

	agentCmd.Register(
		agentInstallCmd,
		agentListCmd,
		agentSearchCmd,
		agentRunCmd,
		agentValidateCmd,
		agentIndexCmd,
		agentInfoCmd,
		agentRemoveCmd,
		agentExtractCmd,
		agentSanitizeCmd,
		agentRefineCmd,
	)
}

// --- install ---

var agentInstallCmd = cli.New(
	cli.Name("install"),
	cli.Short("Install an agent from a GitHub registry"),
	cli.Run(runAgentInstall),
)

func runAgentInstall(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl agent install <agent-id> [--registry owner/repo]")
		fmt.Fprintln(os.Stderr, "  e.g. aboxctl agent install marketing/cro-optimizer")
		os.Exit(1)
	}

	agentID := args[0]

	// Parse registry: could be "owner/repo" or contain a specific agent
	// e.g. "jiusanzhou/my-agents/cool-agent" → registry=jiusanzhou/my-agents, id=cool-agent
	registry := agentRegistry
	if strings.Count(agentID, "/") >= 2 {
		parts := strings.SplitN(agentID, "/", 3)
		registry = parts[0] + "/" + parts[1]
		agentID = parts[2]
	}

	// agentID can be "category/name" (e.g. "marketing/cro-optimizer") or just "name"
	// We'll try both path patterns when fetching from the registry
	agentPath := agentID  // Used for GitHub path: agents/<agentPath>/agent.yaml
	localName := agentID  // Used for local storage directory name
	if strings.Contains(agentID, "/") {
		// e.g. "marketing/cro-optimizer" → path stays, localName = "cro-optimizer"
		parts := strings.SplitN(agentID, "/", 2)
		localName = parts[1]
	}

	fmt.Printf("Installing %s from %s...\n", agentID, registry)

	// Fetch from GitHub API (raw content)
	files := []string{"agent.yaml", "SOUL.md", "AGENTS.md", "IDENTITY.md", "README.md"}
	destDir := filepath.Join(agentDir, localName)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dir: %v\n", err)
		os.Exit(1)
	}

	downloaded := 0
	for _, f := range files {
		// Try agents/<agentPath>/<file> first (nested structure)
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/agents/%s/%s", registry, agentPath, f)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch %s: %v\n", f, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			if f == "agent.yaml" {
				// If agentPath has no slash, try to resolve via index.json
				if !strings.Contains(agentPath, "/") {
					resolved := resolveAgentPath(registry, agentID)
					if resolved != "" {
						agentPath = resolved
						url = fmt.Sprintf("https://raw.githubusercontent.com/%s/main/%s/%s", registry, agentPath, f)
						resp2, err2 := http.Get(url)
						if err2 == nil && resp2.StatusCode == 200 {
							data, _ := io.ReadAll(resp2.Body)
							resp2.Body.Close()
							os.WriteFile(filepath.Join(destDir, f), data, 0644)
							downloaded++
							fmt.Printf("  ✓ %s\n", f)
							continue
						}
						if resp2 != nil {
							resp2.Body.Close()
						}
					}
				}
				fmt.Fprintf(os.Stderr, "Error: agent %q not found in %s\n", agentID, registry)
				fmt.Fprintln(os.Stderr, "Hint: try 'aboxctl agent search' to find available agents")
				os.Exit(1)
			}
			continue // optional file
		}
		if resp.StatusCode != 200 {
			fmt.Fprintf(os.Stderr, "Warning: %s returned %d\n", f, resp.StatusCode)
			continue
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: read %s: %v\n", f, err)
			continue
		}

		if err := os.WriteFile(filepath.Join(destDir, f), data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", f, err)
			os.Exit(1)
		}
		downloaded++
		fmt.Printf("  ✓ %s\n", f)
	}

	// Also try to download experience/ directory via GitHub API
	fetchExperienceDir(registry, agentPath, destDir)

	// Validate
	m, err := agent.ParseFile(filepath.Join(destDir, "agent.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: agent.yaml validation failed: %v\n", err)
	} else {
		fmt.Printf("\nInstalled %s v%s (%s)\n", m.Name, m.Version, m.ID)
		fmt.Printf("  Location: %s\n", destDir)
		if m.Model != nil && m.Model.Recommended != "" {
			fmt.Printf("  Recommended model: %s\n", m.Model.Recommended)
		}
		if m.Experience != nil {
			fmt.Printf("  Experience: %s (%d packs)\n", m.Experience.Level, m.Experience.Packs)
		}
		fmt.Printf("\nRun with: aboxctl agent run %s\n", localName)
	}
}

// resolveAgentPath looks up the agent's path from the registry index.json.
func resolveAgentPath(registry, agentID string) string {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/index.json", registry)
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()

	var index struct {
		Agents []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return ""
	}

	// Try exact match on id
	for _, a := range index.Agents {
		if a.ID == agentID {
			return a.Path
		}
	}
	// Try suffix match (e.g. "cro-optimizer" matches "marketing-cro-optimizer")
	for _, a := range index.Agents {
		if strings.HasSuffix(a.ID, "-"+agentID) || strings.HasSuffix(a.ID, agentID) {
			return a.Path
		}
	}
	return ""
}

// --- run (install + start in one step) ---

var (
	agentRunRuntime  string
	agentRunExecutor string
	agentRunMessage  string
)

var agentRunCmd = cli.New(
	cli.Name("run"),
	cli.Short("Install (if needed) and run an agent from a registry"),
	cli.Run(runAgentRun),
)

func init() {
	agentRunCmd.Flags().StringVar(&agentRunRuntime, "runtime", "", "Override runtime (claude, codex, gemini, etc.)")
	agentRunCmd.Flags().StringVar(&agentRunExecutor, "executor", "", "Override executor (docker, local, e2b, etc.)")
	agentRunCmd.Flags().StringVar(&agentRunMessage, "message", "", "Initial message to send to the agent")
}

func runAgentRun(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl agent run <agent-id> [--runtime claude] [--message 'hello']")
		fmt.Fprintln(os.Stderr, "  e.g. aboxctl agent run marketing/cro-optimizer")
		fmt.Fprintln(os.Stderr, "  e.g. aboxctl agent run cro-optimizer --runtime codex")
		os.Exit(1)
	}

	agentID := args[0]
	localName := agentID
	if strings.Contains(agentID, "/") {
		parts := strings.SplitN(agentID, "/", 2)
		localName = parts[1]
	}

	// Check if already installed locally
	yamlPath := filepath.Join(agentDir, localName, "agent.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		// Not installed — install first
		fmt.Printf("Agent %s not found locally, installing...\n\n", agentID)
		runAgentInstall(cmd, agentID)
		fmt.Println()
	}

	// Parse the installed agent
	m, err := agent.ParseFile(yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing agent: %v\n", err)
		os.Exit(1)
	}

	// Build AGENTS.md content from the installed files
	agentFileContent := buildAgentFileFromInstalled(localName, m)

	// Determine runtime
	runtime := agentRunRuntime
	if runtime == "" {
		runtime = m.PreferredFramework()
	}
	if runtime == "" {
		runtime = "claude" // default
	}

	fmt.Printf("Starting %s (%s) with runtime=%s...\n", m.Name, m.ID, runtime)

	// Create run via API
	client, err := newClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to server: %v\n", err)
		fmt.Fprintln(os.Stderr, "Is the ABox server running? (abox --config config.yaml)")
		os.Exit(1)
	}
	defer client.Close()

	req := map[string]any{
		"name":       m.Name,
		"runtime":    runtime,
		"agent_file": agentFileContent,
		"mode":       "session",
	}
	if agentRunExecutor != "" {
		req["executor"] = agentRunExecutor
	}
	if agentRunMessage != "" {
		req["message"] = agentRunMessage
	}

	var run model.Run
	if err := client.Call(context.Background(), "CreateRun", req, &run); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating run: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Agent started!\n")
	fmt.Printf("  Run ID: %s\n", run.ID)
	fmt.Printf("  Name:   %s\n", m.Name)
	fmt.Printf("  Status: %s\n", run.Status)
	if agentRunMessage != "" {
		fmt.Printf("  Message: %s\n", agentRunMessage)
	}
	fmt.Printf("\nChat with: aboxctl chat --session %s\n", run.ID)
}

// buildAgentFileFromInstalled constructs AGENTS.md content from installed agent files.
func buildAgentFileFromInstalled(localName string, m *agent.Manifest) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", m.Name))
	b.WriteString(fmt.Sprintf("%s\n\n", m.Description))

	// Include SOUL.md content if available
	soulPath := filepath.Join(agentDir, localName, "SOUL.md")
	if soulData, err := os.ReadFile(soulPath); err == nil {
		b.WriteString(string(soulData))
		b.WriteString("\n\n")
	}

	// Include AGENTS.md content if available
	agentsPath := filepath.Join(agentDir, localName, "AGENTS.md")
	if agentsData, err := os.ReadFile(agentsPath); err == nil {
		b.WriteString(string(agentsData))
		b.WriteString("\n\n")
	}

	// Add persona as instructions
	if m.Persona != nil {
		b.WriteString("## Persona\n\n")
		b.WriteString(fmt.Sprintf("**Style:** %s\n\n", m.Persona.Style))
		b.WriteString(fmt.Sprintf("**Tone:** %s\n\n", m.Persona.Tone))
		if len(m.Persona.Principles) > 0 {
			b.WriteString("**Principles:**\n")
			for _, p := range m.Persona.Principles {
				b.WriteString(fmt.Sprintf("- %s\n", p))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// fetchExperienceDir downloads experience files from GitHub.
func fetchExperienceDir(registry, agentID, destDir string) {
	// Use GitHub API to list directory contents
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/agents/%s/experience", registry, agentID)
	resp, err := http.Get(apiURL)
	if err != nil || resp.StatusCode != 200 {
		return // experience dir is optional
	}
	defer resp.Body.Close()

	var contents []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return
	}

	if len(contents) == 0 {
		return
	}

	expDir := filepath.Join(destDir, "experience")
	os.MkdirAll(expDir, 0755)

	for _, f := range contents {
		if f.DownloadURL == "" {
			continue
		}
		resp, err := http.Get(f.DownloadURL)
		if err != nil || resp.StatusCode != 200 {
			continue
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		os.WriteFile(filepath.Join(expDir, f.Name), data, 0644)
		fmt.Printf("  ✓ experience/%s\n", f.Name)
	}
}

// --- list ---

var agentListCmd = cli.New(
	cli.Name("list", "ls"),
	cli.Short("List locally installed agents"),
	cli.Run(runAgentList),
)

func runAgentList(cmd *cli.Command, args ...string) {
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No agents installed.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tVERSION\tLEVEL\tFRAMEWORK")

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		yamlPath := filepath.Join(agentDir, e.Name(), "agent.yaml")
		m, err := agent.ParseFile(yamlPath)
		if err != nil {
			fmt.Fprintf(w, "%s\t(invalid)\t-\t-\t-\n", e.Name())
			continue
		}

		level := "-"
		if m.Experience != nil && m.Experience.Level != "" {
			level = m.Experience.Level
		}
		fw := m.PreferredFramework()
		if fw == "" {
			fw = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", m.ID, m.Name, m.Version, level, fw)
	}
	w.Flush()
}

// --- search ---

var agentSearchCmd = cli.New(
	cli.Name("search"),
	cli.Short("Search agents in a GitHub registry"),
	cli.Run(runAgentSearch),
)

func runAgentSearch(cmd *cli.Command, args ...string) {
	query := ""
	if len(args) > 0 {
		query = strings.ToLower(strings.Join(args, " "))
	}

	// Fetch index.json from registry
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/index.json", agentRegistry)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching index: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "Error: registry index not found (HTTP %d)\n", resp.StatusCode)
		fmt.Fprintln(os.Stderr, "The registry may not have an index.json yet.")
		os.Exit(1)
	}

	// Support both formats:
	// 1. New format: {"agents": [...], "categories": [...], "total": N}
	// 2. Legacy format: [...]
	var entries []IndexEntry

	raw, _ := io.ReadAll(resp.Body)

	// Try new format first
	var newIndex struct {
		Agents []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Emoji       string `json:"emoji"`
			Description string `json:"description"`
			Category    string `json:"category"`
			Path        string `json:"path"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(raw, &newIndex); err == nil && len(newIndex.Agents) > 0 {
		for _, a := range newIndex.Agents {
			entries = append(entries, IndexEntry{
				ID:          a.ID,
				Name:        a.Name,
				Description: a.Description,
				Tags:        []string{a.Category},
			})
		}
	} else {
		// Fall back to legacy flat array format
		_ = json.Unmarshal(raw, &entries)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION")

	count := 0
	for _, e := range entries {
		if query != "" {
			match := strings.Contains(strings.ToLower(e.ID), query) ||
				strings.Contains(strings.ToLower(e.Name), query) ||
				strings.Contains(strings.ToLower(e.Description), query)
			for _, tag := range e.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		desc := e.Description
		if len(desc) > 70 {
			desc = desc[:67] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", e.ID, e.Name, desc)
		count++
	}
	w.Flush()

	if count == 0 && query != "" {
		fmt.Printf("\nNo agents found matching %q\n", query)
	} else {
		fmt.Printf("\n%d agents found\n", count)
	}
}

// IndexEntry represents an agent in the registry index.
type IndexEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Framework   string   `json:"framework,omitempty"`
	Level       string   `json:"level,omitempty"`
}

// --- validate ---

var agentValidateCmd = cli.New(
	cli.Name("validate"),
	cli.Short("Validate an agent.yaml file"),
	cli.Run(runAgentValidate),
)

func runAgentValidate(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl agent validate <agent.yaml>")
		os.Exit(1)
	}

	m, err := agent.ParseFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Valid: %s v%s (%s)\n", m.Name, m.Version, m.ID)
	if m.Persona != nil {
		fmt.Printf("   Style: %s\n", m.Persona.Style)
		fmt.Printf("   Tone:  %s\n", m.Persona.Tone)
	}
	if m.Experience != nil {
		fmt.Printf("   Level: %s (%d packs)\n", m.Experience.Level, m.Experience.Packs)
	}
	if fw := m.PreferredFramework(); fw != "" {
		fmt.Printf("   Framework: %s\n", fw)
	}
}

// --- index ---

var agentIndexCmd = cli.New(
	cli.Name("index"),
	cli.Short("Generate index.json from an agents directory"),
	cli.Run(runAgentIndex),
)

func runAgentIndex(cmd *cli.Command, args ...string) {
	dir := "agents"
	if len(args) > 0 {
		dir = args[0]
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", dir, err)
		os.Exit(1)
	}

	var index []IndexEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		yamlPath := filepath.Join(dir, e.Name(), "agent.yaml")
		m, err := agent.ParseFile(yamlPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", e.Name(), err)
			continue
		}

		entry := IndexEntry{
			ID:          m.ID,
			Name:        m.Name,
			Version:     m.Version,
			Description: m.Description,
			Author:      m.Author,
			Framework:   m.PreferredFramework(),
		}
		if m.Marketplace != nil {
			entry.Tags = m.Marketplace.Tags
		}
		if m.Experience != nil {
			entry.Level = m.Experience.Level
		}
		index = append(index, entry)
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// --- info ---

var agentInfoCmd = cli.New(
	cli.Name("info"),
	cli.Short("Show details of an installed agent"),
	cli.Run(runAgentInfo),
)

func runAgentInfo(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl agent info <agent-id>")
		os.Exit(1)
	}

	yamlPath := filepath.Join(agentDir, args[0], "agent.yaml")
	m, err := agent.ParseFile(yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(m, "", "  ")
	fmt.Println(string(data))
}

// --- remove ---

var agentRemoveCmd = cli.New(
	cli.Name("remove", "rm"),
	cli.Short("Remove a locally installed agent"),
	cli.Run(runAgentRemove),
)

func runAgentRemove(cmd *cli.Command, args ...string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: aboxctl agent remove <agent-id>")
		os.Exit(1)
	}

	dir := filepath.Join(agentDir, args[0])
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Agent %q not found\n", args[0])
		os.Exit(1)
	}

	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed %s\n", args[0])
}
