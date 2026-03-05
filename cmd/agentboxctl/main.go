package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"go.zoe.im/agentbox/internal/model"

	"go.zoe.im/x"
	"go.zoe.im/x/cli"
	"go.zoe.im/x/talk"
	"go.zoe.im/x/version"

	_ "go.zoe.im/x/talk/transport/http/std"
)

var serverAddr = "http://localhost:8080"

func newClient() (*talk.Client, error) {
	if addr := os.Getenv("AGENTBOX_SERVER"); addr != "" {
		serverAddr = addr
	}
	cfg := x.TypedLazyConfig{
		Type:   "http",
		Config: json.RawMessage(fmt.Sprintf(`{"addr":"%s"}`, serverAddr)),
	}
	return talk.NewClientFromConfig(cfg)
}

var cmd = cli.New(
	cli.Name("agentboxctl"),
	cli.Short("AgentBox CLI"),
	version.NewOption(true),
)

var runCmd = cli.New(
	cli.Name("run"),
	cli.Short("Submit an agent run"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: agentboxctl run <AGENTS.md>")
			os.Exit(1)
		}

		data, err := os.ReadFile(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		client, err := newClient()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer client.Close()

		req := map[string]any{
			"name":       args[0],
			"agent_file": string(data),
		}
		var run model.Run
		if err := client.Call(context.Background(), "CreateRun", req, &run); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		fmt.Printf("Run submitted: %s (status: %s)\n", run.ID, run.Status)
	}),
)

var listCmd = cli.New(
	cli.Name("list", "ls"),
	cli.Short("List all runs"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		client, err := newClient()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer client.Close()

		var runs []*model.Run
		if err := client.Call(context.Background(), "ListRuns", nil, &runs); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCREATED")
		for _, r := range runs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.ID, r.Name, r.Status, r.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		w.Flush()
	}),
)

var getCmd = cli.New(
	cli.Name("get"),
	cli.Short("Get run details"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: agentboxctl get <run-id>")
			os.Exit(1)
		}

		client, err := newClient()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer client.Close()

		var run model.Run
		if err := client.Call(context.Background(), "GetRun", args[0], &run); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		data, _ := json.MarshalIndent(run, "", "  ")
		fmt.Println(string(data))
	}),
)

var cancelCmd = cli.New(
	cli.Name("cancel"),
	cli.Short("Cancel a running run"),
	cli.Run(func(cmd *cli.Command, args ...string) {
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: agentboxctl cancel <run-id>")
			os.Exit(1)
		}

		client, err := newClient()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer client.Close()

		if err := client.Call(context.Background(), "DeleteRun", args[0], nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Run cancelled:", args[0])
	}),
)

func main() {
	cmd.Register(runCmd, listCmd, getCmd, cancelCmd)
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
