package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

		addr := serverAddr
		if a := os.Getenv("AGENTBOX_SERVER"); a != "" {
			addr = a
		}

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

		fmt.Println()
		fmt.Println("  \033[1mABox Session\033[0m  \033[33m" + sid + "\033[0m  \033[32m" + session.Status + "\033[0m")
		fmt.Println("  \033[2mType your message. /quit to exit.\033[0m")
		fmt.Println()

		ctx, cancel := context.WithCancel(context.Background())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\n\033[2m  Stopping session...\033[0m")
			req, _ := http.NewRequest("DELETE", addr+"/api/v1/session/"+session.ID, nil)
			http.DefaultClient.Do(req)
			cancel()
			os.Exit(0)
		}()

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("\033[1;32m> \033[0m")
			if !scanner.Scan() {
				break
			}
			msg := strings.TrimSpace(scanner.Text())
			if msg == "" {
				continue
			}
			if msg == "/quit" || msg == "/exit" || msg == "/q" {
				break
			}

			msgBody, _ := json.Marshal(map[string]string{
				"session_id": session.ID,
				"message":    msg,
			})

			fmt.Print("\033[2m  thinking...\033[0m\r")

			req, _ := http.NewRequestWithContext(ctx, "POST",
				addr+"/api/v1/sessionmessage", strings.NewReader(string(msgBody)))
			req.Header.Set("Content-Type", "application/json")

			msgResp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Printf("\033[31m  Error: %v\033[0m\n", err)
				continue
			}

			var result struct {
				Response string `json:"response"`
				Code     int    `json:"code"`
				Message  string `json:"message"`
			}
			json.NewDecoder(msgResp.Body).Decode(&result)
			msgResp.Body.Close()

			fmt.Print("\r\033[K")

			if result.Code != 0 {
				fmt.Printf("\033[31m  Error: %s\033[0m\n\n", result.Message)
			} else {
				fmt.Printf("\033[36m< \033[0m%s\n\n", result.Response)
			}
		}

		fmt.Println("\n\033[2m  Stopping session...\033[0m")
		req, _ := http.NewRequest("DELETE", addr+"/api/v1/session/"+session.ID, nil)
		http.DefaultClient.Do(req)
		fmt.Println("\033[32m  Session ended.\033[0m")
	}),
)
