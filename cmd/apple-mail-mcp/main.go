package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/maximbilan/apple-mail-mcp/internal/installer"
	"github.com/maximbilan/apple-mail-mcp/internal/mail"
	"github.com/maximbilan/apple-mail-mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			if err := runInstall(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "uninstall":
			if err := runUninstall(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "tools-docs":
			if err := runToolsDocs(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}

	if err := runServer(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(args []string) error {
	fs := flag.NewFlagSet("apple-mail-mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	defaultReadOnly := parseEnvBool("APPLE_MAIL_MCP_READ_ONLY")
	defaultLogLevel := strings.TrimSpace(os.Getenv("APPLE_MAIL_MCP_LOG_LEVEL"))
	if defaultLogLevel == "" {
		defaultLogLevel = "info"
	}
	readOnly := fs.Bool("read-only", defaultReadOnly, "register only read-side tools")
	logLevel := fs.String("log-level", defaultLogLevel, "log level: debug|info|warn|error")
	if err := fs.Parse(args); err != nil {
		return err
	}

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	server := mcp.NewServer(&mcp.Implementation{Name: "apple-mail-mcp", Version: version}, nil)
	client := mail.NewClient(mail.NewOsaScriptRunnerFromEnv(), logger)
	tools.Register(server, client, tools.RegisterOptions{ReadOnly: *readOnly, Logger: logger})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("server run failed: %w", err)
	}
	return nil
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("apple-mail-mcp install", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	autoYes := fs.Bool("yes", false, "auto-confirm prompts")
	binaryPath := fs.String("binary-path", "", "path to apple-mail-mcp binary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, err := installer.Install(context.Background(), installer.Options{
		BinaryPath: *binaryPath,
		In:         os.Stdin,
		Out:        os.Stdout,
		Err:        os.Stderr,
		AutoYes:    *autoYes,
	})
	return err
}

func runUninstall(args []string) error {
	fs := flag.NewFlagSet("apple-mail-mcp uninstall", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	autoYes := fs.Bool("yes", false, "auto-confirm prompts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, err := installer.Uninstall(context.Background(), installer.Options{
		In:      os.Stdin,
		Out:     os.Stdout,
		Err:     os.Stderr,
		AutoYes: *autoYes,
	})
	return err
}

func runToolsDocs(args []string) error {
	fs := flag.NewFlagSet("apple-mail-mcp tools-docs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	readOnly := fs.Bool("read-only", false, "generate docs for read-only mode")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println("| Name | Params | Description |")
	fmt.Println("|---|---|---|")
	for _, d := range tools.ToolDocs(*readOnly) {
		fmt.Printf("| %s | %s | %s |\n", d.Name, strings.ReplaceAll(d.Params, "|", "\\|"), strings.ReplaceAll(d.Description, "|", "\\|"))
	}
	return nil
}

func parseLogLevel(raw string) (slog.Leveler, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return nil, fmt.Errorf("invalid --log-level %q", raw)
	}
}

func parseEnvBool(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
