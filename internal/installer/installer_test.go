package installer

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallAndUninstall(t *testing.T) {
	origOS := currentOS
	origNow := timeNow
	currentOS = "darwin"
	timeNow = func() time.Time { return time.Date(2026, 5, 10, 14, 30, 55, 0, time.UTC) }
	t.Cleanup(func() {
		currentOS = origOS
		timeNow = origNow
	})

	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+origPath)

	claudeConfig := filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	if err := os.MkdirAll(filepath.Dir(claudeConfig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeConfig, []byte(`{"mcpServers":{"other":{"command":"x"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	claudeLog := filepath.Join(home, "claude.log")
	if err := writeExecutable(filepath.Join(binDir, "claude"), "#!/bin/sh\necho \"$@\" >> \"$CLAUDE_LOG\"\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_LOG", claudeLog)

	jqScript := `#!/bin/sh
set -eu
if [ "${1:-}" = "--arg" ]; then
  cmd="$3"
  file="$5"
  python3 - "$cmd" "$file" <<'PY'
import json,sys
cmd,file=sys.argv[1],sys.argv[2]
try:
    data=json.load(open(file))
except Exception:
    data={}
if not isinstance(data,dict):
    data={}
data.setdefault("mcpServers", {})["apple-mail"]={"command":cmd,"args":[]}
json.dump(data, sys.stdout)
PY
else
  file="$2"
  python3 - "$file" <<'PY'
import json,sys
file=sys.argv[1]
try:
    data=json.load(open(file))
except Exception:
    data={}
if isinstance(data,dict) and isinstance(data.get("mcpServers"),dict):
    data["mcpServers"].pop("apple-mail",None)
json.dump(data, sys.stdout)
PY
fi
`
	if err := writeExecutable(filepath.Join(binDir, "jq"), jqScript); err != nil {
		t.Fatal(err)
	}

	binaryPath := filepath.Join(home, ".local", "bin", "apple-mail-mcp")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := Install(context.Background(), Options{BinaryPath: binaryPath, AutoYes: true, In: strings.NewReader(""), Out: io.Discard, Err: io.Discard})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if !res.ConfiguredClaudeDesktop || !res.ConfiguredClaudeCode {
		t.Fatalf("expected both clients configured, got %+v", res)
	}

	configAfterInstall, err := os.ReadFile(claudeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configAfterInstall), "apple-mail") || !strings.Contains(string(configAfterInstall), "other") {
		t.Fatalf("unexpected install config: %s", string(configAfterInstall))
	}
	if _, err := os.Stat(claudeConfig + ".bak.20260510-143055"); err != nil {
		t.Fatalf("expected backup file, got: %v", err)
	}

	claudeCalls, _ := os.ReadFile(claudeLog)
	if !strings.Contains(string(claudeCalls), "mcp add apple-mail "+binaryPath+" --scope user") {
		t.Fatalf("unexpected claude install call: %s", string(claudeCalls))
	}

	res, err = Uninstall(context.Background(), Options{AutoYes: true, In: strings.NewReader(""), Out: io.Discard, Err: io.Discard})
	if err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	if !res.ConfiguredClaudeDesktop {
		t.Fatalf("expected desktop cleanup, got %+v", res)
	}

	configAfterUninstall, err := os.ReadFile(claudeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configAfterUninstall), "apple-mail") {
		t.Fatalf("apple-mail should be removed: %s", string(configAfterUninstall))
	}
	if !strings.Contains(string(configAfterUninstall), "other") {
		t.Fatalf("other entries should be preserved: %s", string(configAfterUninstall))
	}

	claudeCalls, _ = os.ReadFile(claudeLog)
	if !strings.Contains(string(claudeCalls), "mcp remove apple-mail") {
		t.Fatalf("unexpected claude uninstall call: %s", string(claudeCalls))
	}
}

func TestInstallNonDarwin(t *testing.T) {
	origOS := currentOS
	currentOS = "linux"
	t.Cleanup(func() { currentOS = origOS })

	_, err := Install(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected non-darwin error")
	}
}

func writeExecutable(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}
