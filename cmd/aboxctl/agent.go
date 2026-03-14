package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

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

	agentCmd.PersistentFlags().StringVar(&agentRegistry, "registry", "abox-agents/registry", "GitHub registry (owner/repo)")
	agentCmd.PersistentFlags().StringVar(&agentDir, "dir", defaultDir, "Local agent store directory")

	agentCmd.Register(
		agentInstallCmd,
		agentListCmd,
		agentSearchCmd,
		agentValidateCmd,
		agentIndexCmd,
		agentInfoCmd,
		agentRemoveCmd,
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

	fmt.Printf("Installing %s from %s...\n", agentID, registry)

	// Fetch from GitHub API (raw content)
	files := []string{"agent.yaml", "SOUL.md", "AGENTS.md", "IDENTITY.md"}
	destDir := filepath.Join(agentDir, agentID)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dir: %v\n", err)
		os.Exit(1)
	}

	downloaded := 0
	for _, f := range files {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/agents/%s/%s", registry, agentID, f)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch %s: %v\n", f, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			if f == "agent.yaml" {
				fmt.Fprintf(os.Stderr, "Error: agent %q not found in %s\n", agentID, registry)
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
	fetchExperienceDir(registry, agentID, destDir)

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
	}
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

	var index []IndexEntry
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing index: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tVERSION\tDESCRIPTION")

	for _, e := range index {
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
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.ID, e.Name, e.Version, desc)
	}
	w.Flush()
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
