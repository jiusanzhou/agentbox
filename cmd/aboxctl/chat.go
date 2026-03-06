package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net"
	"os"
	osExec "os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"
	"syscall"

	"github.com/chzyer/readline"
	"go.zoe.im/x/cli"
)

var chatCmd = cli.New(
	cli.Name("chat"),
	cli.Short("Start an interactive session with an agent"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		agentFile := "You are a helpful coding assistant. Be concise and helpful."
		if len(args) > 0 {
			if data, err := os.ReadFile(args[0]); err == nil {
				agentFile = string(data)
			} else {
				agentFile = strings.Join(args, " ")
			}
		}

		addr := getAddr()

		// Create session via API
		body, _ := json.Marshal(map[string]any{
			"name":       "chat-session",
			"agent_file": agentFile,
		})

		resp, err := http.Post(addr+"/api/v1/session", "application/json",
			strings.NewReader(string(body)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var session struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		json.NewDecoder(resp.Body).Decode(&session)

		if session.ID == "" {
			fmt.Fprintln(os.Stderr, "Failed to create session")
			os.Exit(1)
		}

		sid := session.ID
		if len(sid) > 8 {
			sid = sid[:8]
		}
		containerName := "abox-" + session.ID

		fmt.Println()
		fmt.Printf("  \033[1mABox Session\033[0m  \033[33m%s\033[0m  \033[32m%s\033[0m\n", sid, session.Status)
		fmt.Println("  \033[2mCtrl+C or /quit to exit. Arrow keys for history.\033[0m")
		fmt.Println()

		rl, err := readline.NewEx(&readline.Config{
			Prompt:            "\033[1;32m> \033[0m",
			HistoryFile:       os.ExpandEnv("$HOME/.abox_chat_history"),
			HistorySearchFold: true,
			InterruptPrompt:   "^C",
			EOFPrompt:         "/quit",
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer rl.Close()

		stopSession := func() {
			req, _ := http.NewRequest("DELETE", addr+"/api/v1/session/"+session.ID, nil)
			http.DefaultClient.Do(req)
		}

		ctx, cancel := context.WithCancel(context.Background())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\n\033[2m  Stopping session...\033[0m")
			stopSession()
			cancel()
			rl.Close()
			fmt.Println("\033[32m  Session ended.\033[0m")
			os.Exit(0)
		}()

		// Detect bridge
		bridgeActive := false
		if conn, err := net.DialTimeout("tcp", "localhost:9800", time.Second); err == nil {
			conn.Close()
			bridgeActive = true
			fmt.Println("  [32m✓[0m Bridge detected (MCP + WebDAV)")
			fmt.Println()
		}

		msgCnt := 0

		for {
			line, err := rl.Readline()
			if err != nil {
				break
			}
			msg := strings.TrimSpace(line)
			if msg == "" {
				continue
			}
			if msg == "/quit" || msg == "/exit" || msg == "/q" {
				break
			}
			if msg == "/clear" {
				readline.ClearScreen(rl)
				continue
			}

			// Stream response via docker exec
			claudeArgs := []string{"exec",
				"-u", "agent",
				"-w", "/workspace",
				"-e", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1",
				containerName,
				"claude", "-p", "--dangerously-skip-permissions",
				"--output-format", "stream-json", "--verbose",
			}
			// Inject MCP config if bridge detected
			if bridgeActive {
				claudeArgs = append(claudeArgs, "--mcp-config", "/home/agent/.claude/mcp.json")
			}
			if msgCnt > 0 {
				claudeArgs = append(claudeArgs, "--continue")
			}
			claudeArgs = append(claudeArgs, msg)

			dockerCmd := osExec.CommandContext(ctx, "docker", claudeArgs...)
			stdout, err := dockerCmd.StdoutPipe()
			if err != nil {
				fmt.Printf("\033[31m  Error: %v\033[0m\n", err)
				continue
			}

			if err := dockerCmd.Start(); err != nil {
				fmt.Printf("\033[31m  Error: %v\033[0m\n", err)
				continue
			}

			fmt.Print("\033[36m< \033[0m")

			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

			var once sync.Once
			hasOutput := false

			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}

				var event struct {
					Type  string `json:"type"`
					Event struct {
						Type  string `json:"type"`
						Delta struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"delta"`
					} `json:"event"`
					Result string `json:"result"`
				}

				if err := json.Unmarshal([]byte(line), &event); err != nil {
					continue
				}

				switch event.Type {
				case "stream_event":
					if event.Event.Type == "content_block_delta" && event.Event.Delta.Type == "text_delta" {
						fmt.Print(event.Event.Delta.Text)
						hasOutput = true
					}
				case "result":
					if !hasOutput && event.Result != "" {
						once.Do(func() {
							fmt.Print(event.Result)
						})
					}
				}
			}

			dockerCmd.Wait()
			fmt.Println()
			fmt.Println()
			msgCnt++
		}

		fmt.Println("\n\033[2m  Stopping session...\033[0m")
		stopSession()
		fmt.Println("\033[32m  Session ended.\033[0m")
	}),
)
