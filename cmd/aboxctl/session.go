package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"go.zoe.im/x/cli"
)

var sessionCmd = cli.New(
	cli.Name("session", "ss"),
	cli.Short("Manage sessions"),
)

var sessionListCmd = cli.New(
	cli.Name("list", "ls"),
	cli.Short("List active sessions"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		addr := getAddr()
		resp, err := http.Get(addr + "/api/v1/runs")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var runs []map[string]any
		json.NewDecoder(resp.Body).Decode(&runs)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tMODE\tSTATUS\tCREATED")
		for _, r := range runs {
			mode := str(r, "mode")
			if mode == "" {
				mode = "run"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				str(r, "id"), str(r, "name"), mode, str(r, "status"), str(r, "created_at"))
		}
		w.Flush()
	}),
)

var sessionStopCmd = cli.New(
	cli.Name("stop", "rm"),
	cli.Short("Stop a session"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: aboxctl session stop <session-id>")
			os.Exit(1)
		}
		addr := getAddr()
		req, _ := http.NewRequest("DELETE", addr+"/api/v1/session/"+args[0], nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		resp.Body.Close()
		fmt.Printf("Session %s stopped.\n", args[0])
	}),
)

var sessionSendCmd = cli.New(
	cli.Name("send", "msg"),
	cli.Short("Send a message to a session"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: aboxctl session send <session-id> <message>")
			os.Exit(1)
		}
		addr := getAddr()
		msg := strings.Join(args[1:], " ")
		body, _ := json.Marshal(map[string]string{
			"session_id": args[0],
			"message":    msg,
		})

		resp, err := http.Post(addr+"/api/v1/sessionmessage", "application/json",
			strings.NewReader(string(body)))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var result struct {
			Response string `json:"response"`
			Code     int    `json:"code"`
			Message  string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Code != 0 {
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Message)
			os.Exit(1)
		}
		fmt.Println(result.Response)
	}),
)

var sessionCreateCmd = cli.New(
	cli.Name("create", "new"),
	cli.Short("Create a new session (returns session ID)"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		agentFile := "You are a helpful assistant."
		name := "session"
		if len(args) > 0 {
			if data, err := os.ReadFile(args[0]); err == nil {
				agentFile = string(data)
				name = args[0]
			} else {
				agentFile = strings.Join(args, " ")
			}
		}

		addr := getAddr()
		body, _ := json.Marshal(map[string]any{
			"name":       name,
			"agent_file": agentFile,
		})

		resp, err := http.Post(addr+"/api/v1/session", "application/json",
			strings.NewReader(string(body)))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var session map[string]any
		json.NewDecoder(resp.Body).Decode(&session)

		fmt.Printf("Session created: %s (status: %s)\n", str(session, "id"), str(session, "status"))
	}),
)

func getAddr() string {
	if a := os.Getenv("AGENTBOX_SERVER"); a != "" {
		return a
	}
	return serverAddr
}

func str(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func init() {
	sessionCmd.Register(sessionListCmd, sessionStopCmd, sessionSendCmd, sessionCreateCmd)
}
