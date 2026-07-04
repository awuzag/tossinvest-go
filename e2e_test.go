//go:build e2e

package tossinvest

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestE2EReadOnlySmoke(t *testing.T) {
	if os.Getenv("TOSSINVEST_E2E") != "1" {
		t.Skip("set TOSSINVEST_E2E=1 to run live Toss Invest e2e tests")
	}
	env := Env{}
	if path := os.Getenv("TOSSINVEST_ENV_FILE"); path != "" {
		loaded, err := LoadEnvFile(path)
		if err != nil {
			t.Fatal(err)
		}
		env = loaded
	}
	clientID := firstNonEmpty(env.First("TOSSINVEST_CLIENT_ID", "TOSS_CLIENT_ID"), env["API_KEY"])
	clientSecret := firstNonEmpty(env.First("TOSSINVEST_CLIENT_SECRET", "TOSS_CLIENT_SECRET"), env["SCRET_KEY"])
	if clientID == "" || clientSecret == "" {
		t.Skip("Toss Invest credentials are not configured")
	}
	client, err := New(WithClientID(clientID), WithClientSecret(clientSecret))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := client.Token(ctx); err != nil {
		var oauthErr *OAuthError
		if errors.As(err, &oauthErr) && oauthErr.ErrorCode == "access_denied" && strings.Contains(strings.ToLower(oauthErr.ErrorDescription), "ip address not allowed") {
			t.Skip("Toss Invest rejected the current IP address; add this machine/IP to the app allowlist before live e2e")
		}
		t.Fatal(err)
	}
	symbol := env.First("TOSSINVEST_E2E_SYMBOL")
	if symbol == "" {
		symbol = "005930"
	}
	if _, err := client.Prices(ctx, []string{symbol}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Stocks(ctx, []string{symbol}); err != nil {
		t.Fatal(err)
	}
}
