package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.json")
}

func configArgs(path string, args ...string) []string {
	withConfig := []string{"--config", path}
	return append(withConfig, args...)
}

func enableFeatureForTest(t *testing.T, configPath string, feature string) {
	t.Helper()
	args := []string{"enable", feature}
	if feature == "live-trading" {
		args = append(args, liveTradingConfirmation)
	}
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), configArgs(configPath, args...), &out, &errOut)
	if code != 0 {
		t.Fatalf("enable %s exit=%d err=%s", feature, code, errOut.String())
	}
}

func TestExecutePricesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/prices" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("authorization") != "Bearer token" {
			t.Fatalf("missing auth header")
		}
		_, _ = w.Write([]byte(`{"result":[{"symbol":"005930","lastPrice":"70000","currency":"KRW"}]}`))
	}))
	defer server.Close()

	var out, errOut bytes.Buffer
	code := Execute(context.Background(), configArgs(tempConfigPath(t),
		"--base-url", server.URL,
		"--access-token", "token",
		"--json",
		"prices", "--symbols", "005930",
	), &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "005930") || strings.Contains(out.String(), "token") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestExecuteTokenRedactsAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"secret-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	var out, errOut bytes.Buffer
	code := Execute(context.Background(), configArgs(tempConfigPath(t),
		"--base-url", server.URL,
		"--client-id", "client",
		"--client-secret", "secret",
		"--json",
		"token",
	), &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d err=%s", code, errOut.String())
	}
	if strings.Contains(out.String(), "secret-token") || !strings.Contains(out.String(), "access_token_present") {
		t.Fatalf("token output leaked or missed redaction marker: %s", out.String())
	}
}

func TestLiveTradingCommandRequiresExplicitActivation(t *testing.T) {
	configPath := tempConfigPath(t)
	enableFeatureForTest(t, configPath, "account")
	enableFeatureForTest(t, configPath, "orders")

	var out, errOut bytes.Buffer
	code := Execute(context.Background(), configArgs(configPath,
		"--access-token", "token",
		"order-create", "--symbol", "005930",
	), &out, &errOut)
	if code == 0 {
		t.Fatal("expected non-zero exit for live trading command without activation")
	}
	if !strings.Contains(errOut.String(), "enable live-trading") {
		t.Fatalf("unexpected error: %s", errOut.String())
	}
}

func TestProtectedCommandsRequireExplicitActivation(t *testing.T) {
	cases := []struct {
		name     string
		enabled  []string
		args     []string
		wantText string
	}{
		{name: "account command", args: []string{"--access-token", "token", "accounts"}, wantText: "enable account"},
		{name: "order read command", enabled: []string{"account"}, args: []string{"--access-token", "token", "orders"}, wantText: "enable orders"},
		{name: "live order command", enabled: []string{"account", "orders"}, args: []string{"--access-token", "token", "order-cancel"}, wantText: "enable live-trading"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := tempConfigPath(t)
			for _, feature := range tc.enabled {
				enableFeatureForTest(t, configPath, feature)
			}
			var out, errOut bytes.Buffer
			code := Execute(context.Background(), configArgs(configPath, tc.args...), &out, &errOut)
			if code == 0 {
				t.Fatal("expected non-zero exit")
			}
			if !strings.Contains(errOut.String(), tc.wantText) {
				t.Fatalf("expected %q in error, got %s", tc.wantText, errOut.String())
			}
		})
	}
}

func TestFeatureEnableDisableAndStatus(t *testing.T) {
	configPath := tempConfigPath(t)

	for _, args := range [][]string{
		{"--json", "enable", "account"},
		{"--json", "enable", "orders"},
		{"--json", "enable", "live-trading", liveTradingConfirmation},
		{"--json", "disable", "orders"},
	} {
		var out, errOut bytes.Buffer
		code := Execute(context.Background(), configArgs(configPath, args...), &out, &errOut)
		if code != 0 {
			t.Fatalf("%s exit=%d err=%s", strings.Join(args, " "), code, errOut.String())
		}
		if !json.Valid(out.Bytes()) {
			t.Fatalf("expected JSON output, got %s", out.String())
		}
	}

	var out, errOut bytes.Buffer
	code := Execute(context.Background(), configArgs(configPath, "--json", "features"), &out, &errOut)
	if code != 0 {
		t.Fatalf("features exit=%d err=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"account"`) || !strings.Contains(out.String(), `"live-trading"`) {
		t.Fatalf("unexpected features output: %s", out.String())
	}
}

func TestLiveTradingEnableRequiresConfirmation(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), configArgs(tempConfigPath(t), "enable", "live-trading"), &out, &errOut)
	if code == 0 {
		t.Fatal("expected non-zero exit without live-trading confirmation")
	}
	if !strings.Contains(errOut.String(), liveTradingConfirmation) {
		t.Fatalf("unexpected error: %s", errOut.String())
	}
}

func TestFeatureEnableRequiresDependencies(t *testing.T) {
	t.Run("orders require account", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute(context.Background(), configArgs(tempConfigPath(t), "enable", "orders"), &out, &errOut)
		if code == 0 {
			t.Fatal("expected non-zero exit without account activation")
		}
		if !strings.Contains(errOut.String(), "enable account") {
			t.Fatalf("unexpected error: %s", errOut.String())
		}
	})

	t.Run("live trading requires orders", func(t *testing.T) {
		configPath := tempConfigPath(t)
		enableFeatureForTest(t, configPath, "account")

		var out, errOut bytes.Buffer
		code := Execute(context.Background(), configArgs(configPath, "enable", "live-trading", liveTradingConfirmation), &out, &errOut)
		if code == 0 {
			t.Fatal("expected non-zero exit without order activation")
		}
		if !strings.Contains(errOut.String(), "enable orders") {
			t.Fatalf("unexpected error: %s", errOut.String())
		}
	})
}

func TestExecuteMarketReadCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer token" {
			t.Fatalf("missing auth header for %s", r.URL.Path)
		}
		w.Header().Set("content-type", "application/json")
		switch r.URL.Path {
		case "/api/v1/prices", "/api/v1/stocks", "/api/v1/stocks/005930/warnings", "/api/v1/trades":
			_, _ = w.Write([]byte(`{"result":[]}`))
		default:
			_, _ = w.Write([]byte(`{"result":{}}`))
		}
	}))
	defer server.Close()

	commands := [][]string{
		{"orderbook", "--symbol", "005930"},
		{"trades", "--symbol", "005930", "--count", "2"},
		{"price-limits", "--symbol", "005930"},
		{"candles", "--symbol", "005930", "--interval", "1d", "--count", "2", "--adjusted=false"},
		{"stocks", "--symbols", "005930,AAPL"},
		{"stock-warnings", "--symbol", "005930"},
		{"exchange-rate", "--base", "USD", "--quote", "KRW"},
		{"market-calendar", "--country", "KR", "--date", "2026-07-04"},
		{"market-calendar", "--country", "US", "--date", "2026-07-04"},
	}
	for _, command := range commands {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			args := append(configArgs(tempConfigPath(t), "--base-url", server.URL, "--access-token", "token", "--json"), command...)
			code := Execute(context.Background(), args, &out, &errOut)
			if code != 0 {
				t.Fatalf("exit=%d err=%s", code, errOut.String())
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("expected JSON output, got %s", out.String())
			}
		})
	}
}

func TestExecuteProtectedReadCommandsWithExplicitActivation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authorization") != "Bearer token" {
			t.Fatalf("missing auth header for %s", r.URL.Path)
		}
		w.Header().Set("content-type", "application/json")
		switch r.URL.Path {
		case "/api/v1/accounts", "/api/v1/commissions":
			_, _ = w.Write([]byte(`{"result":[]}`))
		case "/api/v1/orders":
			_, _ = w.Write([]byte(`{"result":{"orders":[],"nextCursor":null,"hasNext":false}}`))
		default:
			_, _ = w.Write([]byte(`{"result":{}}`))
		}
	}))
	defer server.Close()
	configPath := tempConfigPath(t)
	enableFeatureForTest(t, configPath, "account")
	enableFeatureForTest(t, configPath, "orders")

	commands := []struct {
		args []string
	}{
		{args: []string{"accounts"}},
		{args: []string{"holdings", "--account-seq", "7", "--symbol", "005930"}},
		{args: []string{"buying-power", "--account-seq", "7", "--currency", "KRW"}},
		{args: []string{"sellable-quantity", "--account-seq", "7", "--symbol", "005930"}},
		{args: []string{"commissions", "--account-seq", "7"}},
		{args: []string{"orders", "--account-seq", "7", "--status", "OPEN", "--limit", "5"}},
		{args: []string{"order", "--account-seq", "7", "--order-id", "ord-1"}},
	}
	for _, command := range commands {
		t.Run(strings.Join(command.args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			args := configArgs(configPath, "--base-url", server.URL, "--access-token", "token", "--json")
			args = append(args, command.args...)
			code := Execute(context.Background(), args, &out, &errOut)
			if code != 0 {
				t.Fatalf("exit=%d err=%s", code, errOut.String())
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("expected JSON output, got %s", out.String())
			}
		})
	}
}

func TestExecuteLiveTradingCommandsWithExplicitActivation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Tossinvest-Account") != "7" {
			t.Fatalf("missing account header")
		}
		switch r.URL.Path {
		case "/api/v1/orders":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"result":{"orderId":"ord-1","clientOrderId":"client-1"}}`))
		case "/api/v1/orders/ord-1/modify", "/api/v1/orders/ord-1/cancel":
			_, _ = w.Write([]byte(`{"result":{"orderId":"ord-2"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	commands := [][]string{
		{"order-create", "--account-seq", "7", "--symbol", "005930", "--side", "BUY", "--order-type", "LIMIT", "--quantity", "1", "--price", "70000", "--client-order-id", "client-1"},
		{"order-modify", "--account-seq", "7", "--order-id", "ord-1", "--order-type", "LIMIT", "--quantity", "1", "--price", "71000"},
		{"order-cancel", "--account-seq", "7", "--order-id", "ord-1"},
	}
	for _, command := range commands {
		t.Run(command[0], func(t *testing.T) {
			configPath := tempConfigPath(t)
			enableFeatureForTest(t, configPath, "account")
			enableFeatureForTest(t, configPath, "orders")
			enableFeatureForTest(t, configPath, "live-trading")
			var out, errOut bytes.Buffer
			args := append(configArgs(configPath, "--base-url", server.URL, "--access-token", "token", "--json"), command...)
			code := Execute(context.Background(), args, &out, &errOut)
			if code != 0 {
				t.Fatalf("exit=%d err=%s", code, errOut.String())
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("expected JSON output, got %s", out.String())
			}
		})
	}
}

func TestExecuteEnvFileAndErrors(t *testing.T) {
	t.Run("env file", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("authorization") != "Bearer from-file" {
				t.Fatalf("missing file token")
			}
			_, _ = w.Write([]byte(`{"result":[]}`))
		}))
		defer server.Close()
		envPath := t.TempDir() + "/.env"
		if err := os.WriteFile(envPath, []byte("TOSSINVEST_ACCESS_TOKEN=from-file\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		configPath := tempConfigPath(t)
		enableFeatureForTest(t, configPath, "account")
		code := Execute(context.Background(), configArgs(configPath, "--base-url", server.URL, "--env-file", envPath, "--json", "accounts"), &out, &errOut)
		if code != 0 {
			t.Fatalf("exit=%d err=%s", code, errOut.String())
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Execute(context.Background(), configArgs(tempConfigPath(t), "--access-token", "token", "nope"), &out, &errOut)
		if code == 0 || !strings.Contains(errOut.String(), "unknown command") {
			t.Fatalf("unexpected result exit=%d err=%s", code, errOut.String())
		}
	})
}

func TestExecuteHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), configArgs(tempConfigPath(t), "--help"), &out, &errOut)
	if code != 0 {
		t.Fatalf("expected help to exit zero, got %d: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "Commands:") || !strings.Contains(errOut.String(), "enable") || !strings.Contains(errOut.String(), "order-create") {
		t.Fatalf("unexpected help output: %s", errOut.String())
	}
}
