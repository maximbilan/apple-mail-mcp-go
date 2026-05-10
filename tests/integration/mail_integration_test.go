//go:build integration

package integration

import (
	"context"
	"log/slog"
	"runtime"
	"testing"

	"github.com/maximbilan/apple-mail-mcp-go/internal/mail"
)

func TestListAccountsIntegration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("integration test requires macOS")
	}
	client := mail.NewClient(mail.NewOsaScriptRunnerFromEnv(), slog.Default())
	_, err := client.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
}
