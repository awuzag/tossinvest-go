package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	liveTradingConfirmation      = "confirm-live-orders"
	liveTradingActivationWindow  = 15 * time.Minute
	defaultSettingsDirectoryName = "tossinvest"
	defaultSettingsFileName      = "config.json"
)

type cliSettings struct {
	Features cliFeatureSettings `json:"features"`
}

type cliFeatureSettings struct {
	AccountAPIs      bool   `json:"account_apis"`
	OrderAPIs        bool   `json:"order_apis"`
	LiveTradingUntil string `json:"live_trading_until,omitempty"`
}

type featureStatus struct {
	Feature string `json:"feature"`
	Enabled bool   `json:"enabled"`
	Until   string `json:"until,omitempty"`
}

type featureStatusOutput struct {
	Config   string          `json:"config"`
	Features []featureStatus `json:"features"`
}

func (r *runner) loadSettings() error {
	path, err := resolveSettingsPath(r.options.configPath)
	if err != nil {
		return err
	}
	settings, err := loadCLISettings(path)
	if err != nil {
		return err
	}
	r.settingsPath = path
	r.settings = settings
	return nil
}

func resolveSettingsPath(explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("TOSSINVEST_CONFIG")); path != "" {
		return path, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, defaultSettingsDirectoryName, defaultSettingsFileName), nil
}

func loadCLISettings(path string) (cliSettings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cliSettings{}, nil
	}
	if err != nil {
		return cliSettings{}, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return cliSettings{}, nil
	}
	var settings cliSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return cliSettings{}, fmt.Errorf("read CLI config %s: %w", path, err)
	}
	return settings, nil
}

func saveCLISettings(path string, settings cliSettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (r *runner) runSettingsCommand(args []string) error {
	switch args[0] {
	case "features":
		if len(args) != 1 && !(len(args) == 2 && args[1] == "status") {
			return fmt.Errorf("usage: tossinvest features")
		}
		return r.writeFeatureStatuses()
	case "enable":
		return r.updateFeature(args, true)
	case "disable":
		return r.updateFeature(args, false)
	default:
		return fmt.Errorf("unknown settings command: %s", args[0])
	}
}

func (r *runner) updateFeature(args []string, enabled bool) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tossinvest %s <account|orders|live-trading>", args[0])
	}
	feature, err := normalizeFeature(args[1])
	if err != nil {
		return err
	}
	if feature == "live-trading" && enabled {
		if len(args) != 3 || args[2] != liveTradingConfirmation {
			return fmt.Errorf("live-trading can create, modify, or cancel real orders; run `tossinvest enable live-trading %s` after confirming the request", liveTradingConfirmation)
		}
	} else if len(args) != 2 {
		return fmt.Errorf("usage: tossinvest %s <account|orders|live-trading>", args[0])
	}
	if enabled {
		if feature == "orders" && !r.settings.Features.AccountAPIs {
			return fmt.Errorf("orders require account APIs; run `tossinvest enable account` first")
		}
		if feature == "live-trading" && (!r.settings.Features.AccountAPIs || !r.settings.Features.OrderAPIs) {
			return fmt.Errorf("live-trading requires account and order APIs; run `tossinvest enable account` and `tossinvest enable orders` first")
		}
	}

	settings := r.settings
	status := setFeature(&settings, feature, enabled, r.now())
	if err := saveCLISettings(r.settingsPath, settings); err != nil {
		return err
	}
	r.settings = settings
	return r.writeFeatureUpdate(status)
}

func normalizeFeature(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "account", "accounts", "account-api", "account-apis":
		return "account", nil
	case "order", "orders", "order-api", "order-apis":
		return "orders", nil
	case "live", "live-trading", "live-trading-api", "live-trading-apis":
		return "live-trading", nil
	default:
		return "", fmt.Errorf("unknown feature %q; expected account, orders, or live-trading", name)
	}
}

func setFeature(settings *cliSettings, feature string, enabled bool, now time.Time) featureStatus {
	switch feature {
	case "account":
		settings.Features.AccountAPIs = enabled
		if !enabled {
			settings.Features.OrderAPIs = false
			settings.Features.LiveTradingUntil = ""
		}
	case "orders":
		settings.Features.OrderAPIs = enabled
		if !enabled {
			settings.Features.LiveTradingUntil = ""
		}
	case "live-trading":
		if enabled {
			settings.Features.LiveTradingUntil = now.Add(liveTradingActivationWindow).Format(time.RFC3339)
		} else {
			settings.Features.LiveTradingUntil = ""
		}
	}
	return settings.featureStatus(feature, now)
}

func (settings cliSettings) featureStatus(feature string, now time.Time) featureStatus {
	switch feature {
	case "account":
		return featureStatus{Feature: "account", Enabled: settings.Features.AccountAPIs}
	case "orders":
		return featureStatus{Feature: "orders", Enabled: settings.Features.OrderAPIs}
	case "live-trading":
		enabled := settings.liveTradingEnabled(now)
		status := featureStatus{Feature: "live-trading", Enabled: enabled}
		if enabled {
			status.Until = settings.Features.LiveTradingUntil
		}
		return status
	default:
		return featureStatus{Feature: feature}
	}
}

func (settings cliSettings) featureStatuses(now time.Time) []featureStatus {
	return []featureStatus{
		settings.featureStatus("account", now),
		settings.featureStatus("orders", now),
		settings.featureStatus("live-trading", now),
	}
}

func (settings cliSettings) liveTradingEnabled(now time.Time) bool {
	if strings.TrimSpace(settings.Features.LiveTradingUntil) == "" {
		return false
	}
	until, err := time.Parse(time.RFC3339, settings.Features.LiveTradingUntil)
	if err != nil {
		return false
	}
	return now.Before(until)
}

func (r *runner) writeFeatureStatuses() error {
	statuses := r.settings.featureStatuses(r.now())
	if r.options.jsonOutput {
		return r.write(featureStatusOutput{Config: r.settingsPath, Features: statuses})
	}
	_, _ = fmt.Fprintf(r.out, "config\t%s\n", r.settingsPath)
	for _, status := range statuses {
		_, _ = fmt.Fprintf(r.out, "%s\t%s", status.Feature, enabledLabel(status.Enabled))
		if status.Until != "" {
			_, _ = fmt.Fprintf(r.out, "\tuntil=%s", status.Until)
		}
		_, _ = fmt.Fprintln(r.out)
	}
	return nil
}

func (r *runner) writeFeatureUpdate(status featureStatus) error {
	if r.options.jsonOutput {
		return r.write(struct {
			Config  string `json:"config"`
			Feature string `json:"feature"`
			Enabled bool   `json:"enabled"`
			Until   string `json:"until,omitempty"`
		}{
			Config:  r.settingsPath,
			Feature: status.Feature,
			Enabled: status.Enabled,
			Until:   status.Until,
		})
	}
	_, _ = fmt.Fprintf(r.out, "%s\t%s\n", status.Feature, enabledLabel(status.Enabled))
	if status.Until != "" {
		_, _ = fmt.Fprintf(r.out, "until\t%s\n", status.Until)
	}
	_, _ = fmt.Fprintf(r.out, "config\t%s\n", r.settingsPath)
	return nil
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
