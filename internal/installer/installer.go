package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	serverName = "apple-mail"
)

var (
	currentOS = runtime.GOOS
	timeNow   = time.Now
)

type Options struct {
	BinaryPath string
	In         io.Reader
	Out        io.Writer
	Err        io.Writer
	AutoYes    bool
}

type Result struct {
	ConfiguredClaudeDesktop bool
	ConfiguredClaudeCode    bool
	DetectedClaudeDesktop   bool
	DetectedClaudeCode      bool
}

func Install(ctx context.Context, opts Options) (Result, error) {
	if currentOS != "darwin" {
		return Result{}, errors.New("install is only supported on macOS")
	}
	opts = withDefaults(opts)

	binaryPath, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return Result{}, err
	}

	desktopConfig := claudeDesktopConfigPath()
	desktopDetected := fileExists(desktopConfig)
	_, claudeOnPath := lookPath("claude")

	result := Result{DetectedClaudeDesktop: desktopDetected, DetectedClaudeCode: claudeOnPath}
	if !desktopDetected && !claudeOnPath {
		fmt.Fprintln(opts.Out, "No supported client detected (Claude Desktop or Claude Code).")
		return result, nil
	}

	if desktopDetected && confirm(opts, "Register with Claude Desktop? [Y/n]: ") {
		if err := ensureJQ(ctx, opts); err != nil {
			return result, err
		}
		if err := upsertClaudeDesktopConfig(ctx, opts, desktopConfig, binaryPath); err != nil {
			return result, err
		}
		result.ConfiguredClaudeDesktop = true
	}

	if claudeOnPath && confirm(opts, "Register with Claude Code? [Y/n]: ") {
		_ = runCommandQuiet(ctx, opts, "claude", "mcp", "remove", serverName)
		if err := runCommand(ctx, opts, "claude", "mcp", "add", serverName, binaryPath, "--scope", "user"); err != nil {
			return result, fmt.Errorf("claude mcp add failed: %w", err)
		}
		result.ConfiguredClaudeCode = true
	}

	printInstallNextSteps(opts.Out, result)
	return result, nil
}

func Uninstall(ctx context.Context, opts Options) (Result, error) {
	if currentOS != "darwin" {
		return Result{}, errors.New("uninstall is only supported on macOS")
	}
	opts = withDefaults(opts)
	result := Result{}

	desktopConfig := claudeDesktopConfigPath()
	if fileExists(desktopConfig) {
		result.DetectedClaudeDesktop = true
		if err := ensureJQ(ctx, opts); err != nil {
			return result, err
		}
		if err := removeClaudeDesktopConfig(ctx, opts, desktopConfig); err != nil {
			return result, err
		}
		result.ConfiguredClaudeDesktop = true
	}

	if _, ok := lookPath("claude"); ok {
		result.DetectedClaudeCode = true
		if err := runCommand(ctx, opts, "claude", "mcp", "remove", serverName); err != nil {
			fmt.Fprintf(opts.Err, "warning: claude mcp remove failed: %v\n", err)
		} else {
			result.ConfiguredClaudeCode = true
		}
	}

	fmt.Fprintln(opts.Out, "Uninstall completed.")
	return result, nil
}

func withDefaults(opts Options) Options {
	if opts.In == nil {
		opts.In = os.Stdin
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Err == nil {
		opts.Err = os.Stderr
	}
	return opts
}

func resolveBinaryPath(bin string) (string, error) {
	if strings.TrimSpace(bin) != "" {
		return filepath.Abs(bin)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Abs(exe)
}

func claudeDesktopConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
}

func ensureJQ(ctx context.Context, opts Options) error {
	if _, ok := lookPath("jq"); ok {
		return nil
	}
	if !confirm(opts, "jq is required to edit Claude Desktop config. Install jq with Homebrew now? [Y/n]: ") {
		return errors.New("jq is required but not installed")
	}
	if _, ok := lookPath("brew"); !ok {
		return errors.New("Homebrew not found; install jq manually and retry")
	}
	if err := runCommand(ctx, opts, "brew", "install", "jq"); err != nil {
		return fmt.Errorf("install jq: %w", err)
	}
	if _, ok := lookPath("jq"); !ok {
		return errors.New("jq installation did not succeed")
	}
	return nil
}

func upsertClaudeDesktopConfig(ctx context.Context, opts Options, configPath, binaryPath string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if !fileExists(configPath) {
		if err := os.WriteFile(configPath, []byte("{}\n"), 0o644); err != nil {
			return fmt.Errorf("create config file: %w", err)
		}
	}
	if err := backupFile(configPath); err != nil {
		return err
	}

	filter := `(.mcpServers //= {}) | .mcpServers["apple-mail"] = {"command": $cmd, "args": []}`
	updated, err := runCommandOutput(ctx, opts, "jq", "--arg", "cmd", binaryPath, filter, configPath)
	if err != nil {
		return fmt.Errorf("merge claude_desktop_config.json: %w", err)
	}
	if err := os.WriteFile(configPath, []byte(updated+"\n"), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func removeClaudeDesktopConfig(ctx context.Context, opts Options, configPath string) error {
	if !fileExists(configPath) {
		return nil
	}
	if err := backupFile(configPath); err != nil {
		return err
	}
	filter := `if .mcpServers then .mcpServers |= del(."apple-mail") else . end`
	updated, err := runCommandOutput(ctx, opts, "jq", filter, configPath)
	if err != nil {
		return fmt.Errorf("update claude_desktop_config.json: %w", err)
	}
	if err := os.WriteFile(configPath, []byte(updated+"\n"), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func printInstallNextSteps(w io.Writer, result Result) {
	if result.ConfiguredClaudeDesktop {
		fmt.Fprintln(w, "Restart Claude Desktop.")
	}
	if result.ConfiguredClaudeCode {
		fmt.Fprintln(w, "Run `claude mcp list` to verify.")
	}
	if !result.ConfiguredClaudeDesktop && !result.ConfiguredClaudeCode {
		fmt.Fprintln(w, "No changes were made.")
	}
}

func confirm(opts Options, prompt string) bool {
	if opts.AutoYes {
		fmt.Fprintln(opts.Out, prompt+" yes")
		return true
	}
	fmt.Fprint(opts.Out, prompt)
	reader := bufio.NewReader(opts.In)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == "yes"
}

func backupFile(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config for backup: %w", err)
	}
	stamp := timeNow().Format("20060102-150405")
	backupPath := path + ".bak." + stamp
	if err := os.WriteFile(backupPath, contents, 0o644); err != nil {
		return fmt.Errorf("write backup file: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func lookPath(bin string) (string, bool) {
	p, err := exec.LookPath(bin)
	if err != nil {
		return "", false
	}
	return p, true
}

func runCommand(ctx context.Context, opts Options, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = opts.Out
	cmd.Stderr = opts.Err
	cmd.Stdin = opts.In
	return cmd.Run()
}

func runCommandQuiet(ctx context.Context, opts Options, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = opts.In
	return cmd.Run()
}

func runCommandOutput(ctx context.Context, opts Options, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
