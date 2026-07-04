package tossinvest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("TOSSINVEST_CLIENT_ID='client'\n# comment\nTOSSINVEST_ACCOUNT_SEQ=7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env, err := LoadEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if env.First("TOSSINVEST_CLIENT_ID") != "client" {
		t.Fatalf("unexpected client id: %q", env.First("TOSSINVEST_CLIENT_ID"))
	}
	if env.First("TOSSINVEST_ACCOUNT_SEQ") != "7" {
		t.Fatalf("unexpected account seq: %q", env.First("TOSSINVEST_ACCOUNT_SEQ"))
	}
}
