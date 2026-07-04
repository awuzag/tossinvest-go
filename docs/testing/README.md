# Testing

Default tests do not call live Toss Invest APIs.

```sh
go test ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

The current target is at least 70% statement coverage. The initial implementation covers SDK and CLI command routing with fake HTTP servers.

## Local Scenario E2E

`scenario_e2e_test.go` runs end-to-end workflows against a fake Toss Invest-compatible HTTP server:

- market data discovery: OAuth token, stocks, prices, orderbook, trades, candles
- account and order lifecycle: accounts, holdings, buying power, sellable quantity, commissions, orders, create, detail, modify, cancel

These tests prove the SDK request sequence, headers, query strings, envelopes, and JSON body shapes for all API groups without touching live trading systems.

## Live E2E

Read-only live e2e smoke tests are behind the `e2e` build tag and `TOSSINVEST_E2E=1`.

```sh
TOSSINVEST_E2E=1 \
TOSSINVEST_ENV_FILE=.env.toss \
go test -tags=e2e -run TestE2EReadOnlySmoke -count=1 -v .
```

The repo also provides a wrapper that checks the env file, prints only key names, shows the current public IP when `curl` is available, and exits with code `2` when the live check is blocked by IP allowlisting:

```sh
scripts/e2e-readonly.sh
task e2e:readonly
```

This smoke test issues an OAuth token and calls read-only quote and stock APIs.
The awuzag local env file currently uses `API_KEY` and `SCRET_KEY`; these are supported as file-local aliases.

If Toss Invest returns `access_denied` with `IP address not allowed`, the test skips with an allowlist message. Add the current machine/IP to the Toss Invest app settings, then rerun the same command.

## Protected Account and Order Boundary

Account and order APIs are disabled by default in both SDK and CLI. Mock SDK integration tests enable them explicitly with `WithAccountAPIsEnabled`, `WithOrderAPIsEnabled`, and `WithLiveTradingEnabled` where needed.

The CLI stores feature opt-ins in a local config file. It refuses account commands until account APIs are enabled, refuses order read commands until account and order APIs are enabled, and refuses live trading commands until a recent live-trading activation exists:

```sh
tossinvest enable account
tossinvest enable orders
tossinvest enable live-trading confirm-live-orders
tossinvest --env-file .env.toss \
  order-create --account-seq 1 --symbol 005930 --side BUY --order-type LIMIT --quantity 1 --price 70000
```

The live-trading activation expires after 15 minutes. Tests use a temporary `--config` path so local user settings cannot change test results.

Do not add automatic live trading e2e tests without an explicit user request and a separate safety plan.
