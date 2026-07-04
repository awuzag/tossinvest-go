package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	tossinvest "github.com/awuzag/tossinvest-go"
	tossapi "github.com/awuzag/tossinvest-go/internal/generated/tossapi"
)

type rootOptions struct {
	envFile      string
	baseURL      string
	clientID     string
	clientSecret string
	accessToken  string
	accountSeq   string
	jsonOutput   bool
	configPath   string
}

type runner struct {
	out          io.Writer
	errOut       io.Writer
	options      rootOptions
	settings     cliSettings
	settingsPath string
	now          func() time.Time
}

func Execute(ctx context.Context, args []string, out io.Writer, errOut io.Writer) int {
	r := &runner{out: out, errOut: errOut, now: time.Now}
	if err := r.run(ctx, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		_, _ = fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

func (r *runner) run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tossinvest", flag.ContinueOnError)
	fs.SetOutput(r.errOut)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(r.errOut, "Usage: tossinvest [global flags] <command> [command flags]")
		_, _ = fmt.Fprintln(r.errOut)
		_, _ = fmt.Fprintln(r.errOut, "Commands:")
		for _, line := range commandUsageLines() {
			_, _ = fmt.Fprintln(r.errOut, line)
		}
		_, _ = fmt.Fprintln(r.errOut)
		_, _ = fmt.Fprintln(r.errOut, "Global flags:")
		fs.PrintDefaults()
	}
	fs.StringVar(&r.options.envFile, "env-file", "", "env file path")
	fs.StringVar(&r.options.baseURL, "base-url", "", "Toss Invest OpenAPI base URL")
	fs.StringVar(&r.options.clientID, "client-id", "", "OAuth client ID")
	fs.StringVar(&r.options.clientSecret, "client-secret", "", "OAuth client secret")
	fs.StringVar(&r.options.accessToken, "access-token", "", "existing OAuth access token")
	fs.StringVar(&r.options.accountSeq, "account-seq", "", "accountSeq for account-scoped APIs")
	fs.BoolVar(&r.options.jsonOutput, "json", false, "print JSON output")
	fs.StringVar(&r.options.configPath, "config", "", "CLI config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	remaining := fs.Args()
	if len(remaining) == 0 {
		fs.Usage()
		return flag.ErrHelp
	}
	if err := r.loadSettings(); err != nil {
		return err
	}
	if isSettingsCommand(remaining[0]) {
		return r.runSettingsCommand(remaining)
	}
	if err := r.requireCommandEnabled(remaining[0]); err != nil {
		return err
	}

	env := tossinvest.Env{}
	if r.options.envFile != "" {
		loaded, err := tossinvest.LoadEnvFile(r.options.envFile)
		if err != nil {
			return err
		}
		env = loaded
	}
	r.applyEnv(env)

	client, err := r.client()
	if err != nil {
		return err
	}

	switch remaining[0] {
	case "token":
		token, err := client.Token(ctx)
		if err != nil {
			return err
		}
		return r.write(map[string]any{"token_type": token.TokenType, "expires_in": token.ExpiresIn, "access_token_present": token.AccessToken != ""})
	case "accounts":
		return r.call(func() (any, error) { return client.Accounts(ctx) })
	case "prices":
		fs := commandFlagSet("prices", r.errOut)
		symbols := fs.String("symbols", "", "comma-separated symbols")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) { return client.Prices(ctx, tossinvest.SplitSymbols(*symbols)) })
	case "orderbook":
		fs := commandFlagSet("orderbook", r.errOut)
		symbol := fs.String("symbol", "", "symbol")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) { return client.Orderbook(ctx, *symbol) })
	case "trades":
		fs := commandFlagSet("trades", r.errOut)
		symbol := fs.String("symbol", "", "symbol")
		count := fs.Int("count", 50, "trade count")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) { return client.Trades(ctx, *symbol, *count) })
	case "price-limits":
		fs := commandFlagSet("price-limits", r.errOut)
		symbol := fs.String("symbol", "", "symbol")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) { return client.PriceLimit(ctx, *symbol) })
	case "candles":
		fs := commandFlagSet("candles", r.errOut)
		symbol := fs.String("symbol", "", "symbol")
		interval := fs.String("interval", "1d", "interval: 1m or 1d")
		count := fs.Int("count", 100, "candle count")
		before := fs.String("before", "", "exclusive upper bound")
		adjusted := fs.Bool("adjusted", true, "adjusted price")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.Candles(ctx, tossinvest.CandlesRequest{Symbol: *symbol, Interval: *interval, Count: *count, Before: *before, Adjusted: adjusted})
		})
	case "stocks":
		fs := commandFlagSet("stocks", r.errOut)
		symbols := fs.String("symbols", "", "comma-separated symbols")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) { return client.Stocks(ctx, tossinvest.SplitSymbols(*symbols)) })
	case "stock-warnings":
		fs := commandFlagSet("stock-warnings", r.errOut)
		symbol := fs.String("symbol", "", "symbol")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) { return client.StockWarnings(ctx, *symbol) })
	case "exchange-rate":
		fs := commandFlagSet("exchange-rate", r.errOut)
		base := fs.String("base", "USD", "base currency")
		quote := fs.String("quote", "KRW", "quote currency")
		dateTime := fs.String("date-time", "", "ISO date-time")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.ExchangeRate(ctx, tossinvest.ExchangeRateRequest{BaseCurrency: tossinvest.Currency(*base), QuoteCurrency: tossinvest.Currency(*quote), DateTime: *dateTime})
		})
	case "market-calendar":
		fs := commandFlagSet("market-calendar", r.errOut)
		country := fs.String("country", "KR", "KR or US")
		date := fs.String("date", "", "YYYY-MM-DD")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		if strings.EqualFold(*country, "US") {
			return r.call(func() (any, error) { return client.UsMarketCalendar(ctx, *date) })
		}
		return r.call(func() (any, error) { return client.KrMarketCalendar(ctx, *date) })
	case "holdings":
		fs := commandFlagSet("holdings", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		symbol := fs.String("symbol", "", "optional symbol")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.Holdings(ctx, firstNonEmpty(*account, r.options.accountSeq), *symbol)
		})
	case "orders":
		fs := commandFlagSet("orders", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		status := fs.String("status", "OPEN", "OPEN or CLOSED")
		symbol := fs.String("symbol", "", "symbol")
		limit := fs.Int("limit", 20, "limit")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.Orders(ctx, tossinvest.OrdersRequest{AccountSeq: firstNonEmpty(*account, r.options.accountSeq), Status: *status, Symbol: *symbol, Limit: *limit})
		})
	case "order":
		fs := commandFlagSet("order", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		orderID := fs.String("order-id", "", "order ID")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) { return client.Order(ctx, firstNonEmpty(*account, r.options.accountSeq), *orderID) })
	case "buying-power":
		fs := commandFlagSet("buying-power", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		currency := fs.String("currency", "KRW", "currency")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.BuyingPower(ctx, firstNonEmpty(*account, r.options.accountSeq), tossinvest.Currency(*currency))
		})
	case "sellable-quantity":
		fs := commandFlagSet("sellable-quantity", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		symbol := fs.String("symbol", "", "symbol")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.SellableQuantity(ctx, firstNonEmpty(*account, r.options.accountSeq), *symbol)
		})
	case "commissions":
		fs := commandFlagSet("commissions", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		if err := fs.Parse(remaining[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) { return client.Commissions(ctx, firstNonEmpty(*account, r.options.accountSeq)) })
	case "order-create", "order-modify", "order-cancel":
		return r.runLiveOrder(ctx, client, remaining)
	default:
		return fmt.Errorf("unknown command: %s", remaining[0])
	}
}

func (r *runner) runLiveOrder(ctx context.Context, client *tossinvest.Client, args []string) error {
	switch args[0] {
	case "order-create":
		fs := commandFlagSet("order-create", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		symbol := fs.String("symbol", "", "symbol")
		side := fs.String("side", "BUY", "BUY or SELL")
		orderType := fs.String("order-type", "LIMIT", "LIMIT or MARKET")
		quantity := fs.String("quantity", "", "quantity")
		price := fs.String("price", "", "price")
		amount := fs.String("order-amount", "", "US market amount")
		clientOrderID := fs.String("client-order-id", "", "idempotency key")
		confirm := fs.Bool("confirm-high-value", false, "confirm high-value order")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.CreateOrder(ctx, firstNonEmpty(*account, r.options.accountSeq), tossinvest.OrderCreateRequest{
				ClientOrderID: *clientOrderID, Symbol: *symbol, Side: *side, OrderType: *orderType,
				Quantity: *quantity, Price: *price, OrderAmount: *amount, ConfirmHighValue: *confirm,
			})
		})
	case "order-modify":
		fs := commandFlagSet("order-modify", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		orderID := fs.String("order-id", "", "order ID")
		orderType := fs.String("order-type", "LIMIT", "LIMIT or MARKET")
		quantity := fs.String("quantity", "", "quantity")
		price := fs.String("price", "", "price")
		confirm := fs.Bool("confirm-high-value", false, "confirm high-value order")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.ModifyOrder(ctx, firstNonEmpty(*account, r.options.accountSeq), *orderID, tossinvest.OrderModifyRequest{
				OrderType: *orderType, Quantity: *quantity, Price: *price, ConfirmHighValue: *confirm,
			})
		})
	case "order-cancel":
		fs := commandFlagSet("order-cancel", r.errOut)
		account := fs.String("account-seq", "", "accountSeq")
		orderID := fs.String("order-id", "", "order ID")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return r.call(func() (any, error) {
			return client.CancelOrder(ctx, firstNonEmpty(*account, r.options.accountSeq), *orderID)
		})
	default:
		return fmt.Errorf("unknown live order command: %s", args[0])
	}
}

func (r *runner) applyEnv(env tossinvest.Env) {
	r.options.clientID = firstNonEmpty(r.options.clientID, env.First("TOSSINVEST_CLIENT_ID", "TOSS_CLIENT_ID"), env["API_KEY"])
	r.options.clientSecret = firstNonEmpty(r.options.clientSecret, env.First("TOSSINVEST_CLIENT_SECRET", "TOSS_CLIENT_SECRET"), env["SCRET_KEY"])
	r.options.accessToken = firstNonEmpty(r.options.accessToken, env.First("TOSSINVEST_ACCESS_TOKEN", "TOSS_ACCESS_TOKEN"))
	r.options.accountSeq = firstNonEmpty(r.options.accountSeq, env.First("TOSSINVEST_ACCOUNT_SEQ", "TOSS_ACCOUNT_SEQ"))
}

func (r *runner) client() (*tossinvest.Client, error) {
	options := []tossinvest.Option{
		tossinvest.WithClientID(r.options.clientID),
		tossinvest.WithClientSecret(r.options.clientSecret),
		tossinvest.WithAccessToken(r.options.accessToken),
		tossinvest.WithAccountSeq(r.options.accountSeq),
	}
	if r.options.baseURL != "" {
		options = append(options, tossinvest.WithBaseURL(r.options.baseURL))
	}
	if r.settings.Features.AccountAPIs {
		options = append(options, tossinvest.WithAccountAPIsEnabled())
	}
	if r.settings.Features.OrderAPIs {
		options = append(options, tossinvest.WithOrderAPIsEnabled())
	}
	if r.settings.liveTradingEnabled(r.now()) {
		options = append(options, tossinvest.WithLiveTradingEnabled())
	}
	return tossinvest.New(options...)
}

func (r *runner) call(fn func() (any, error)) error {
	result, err := fn()
	if err != nil {
		return err
	}
	return r.write(result)
}

func (r *runner) write(value any) error {
	if r.options.jsonOutput {
		enc := json.NewEncoder(r.out)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	switch value := value.(type) {
	case []tossinvest.Account:
		for _, account := range value {
			_, _ = fmt.Fprintf(r.out, "%s\t%d\t%s\n", account.AccountNo, account.AccountSeq, account.AccountType)
		}
	case map[string]any:
		for key, val := range value {
			_, _ = fmt.Fprintf(r.out, "%s\t%v\n", key, val)
		}
	default:
		encoded, _ := json.Marshal(value)
		_, _ = fmt.Fprintln(r.out, string(encoded))
	}
	return nil
}

func commandFlagSet(name string, errOut io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	return fs
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

var commandByOperation = map[string]string{
	tossapi.OperationIssueOAuth2Token:    "token",
	tossapi.OperationGetAccounts:         "accounts",
	tossapi.OperationGetOrderbook:        "orderbook",
	tossapi.OperationGetPrices:           "prices",
	tossapi.OperationGetTrades:           "trades",
	tossapi.OperationGetPriceLimit:       "price-limits",
	tossapi.OperationGetCandles:          "candles",
	tossapi.OperationGetStocks:           "stocks",
	tossapi.OperationGetStockWarnings:    "stock-warnings",
	tossapi.OperationGetExchangeRate:     "exchange-rate",
	tossapi.OperationGetKRMarketCalendar: "market-calendar",
	tossapi.OperationGetUSMarketCalendar: "market-calendar",
	tossapi.OperationGetHoldings:         "holdings",
	tossapi.OperationGetOrders:           "orders",
	tossapi.OperationGetOrder:            "order",
	tossapi.OperationCreateOrder:         "order-create",
	tossapi.OperationModifyOrder:         "order-modify",
	tossapi.OperationCancelOrder:         "order-cancel",
	tossapi.OperationGetBuyingPower:      "buying-power",
	tossapi.OperationGetSellableQuantity: "sellable-quantity",
	tossapi.OperationGetCommissions:      "commissions",
}

func commandUsageLines() []string {
	regular, account, order, live := catalogCommands()
	return []string{
		"  features, enable, disable",
		"  " + strings.Join(regular, ", "),
		"  " + strings.Join(account, ", ") + " (enable with: tossinvest enable account)",
		"  " + strings.Join(order, ", ") + " (enable with: tossinvest enable account; tossinvest enable orders)",
		"  " + strings.Join(live, ", ") + " (enable with: tossinvest enable live-trading confirm-live-orders)",
	}
}

func (r *runner) requireCommandEnabled(command string) error {
	missing := []string{}
	if (isAccountCommand(command) || isOrderCommand(command) || isLiveTradingCommand(command)) && !r.settings.Features.AccountAPIs {
		missing = append(missing, "tossinvest enable account")
	}
	if (isOrderCommand(command) || isLiveTradingCommand(command)) && !r.settings.Features.OrderAPIs {
		missing = append(missing, "tossinvest enable orders")
	}
	if isLiveTradingCommand(command) && !r.settings.liveTradingEnabled(r.now()) {
		missing = append(missing, "tossinvest enable live-trading confirm-live-orders")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s is disabled by default; run %s first", command, joinQuotedCommands(missing))
}

func catalogCommands() ([]string, []string, []string, []string) {
	regular := []string{}
	account := []string{}
	order := []string{}
	live := []string{}
	seen := map[string]bool{}
	for _, operation := range tossapi.Operations() {
		command := commandByOperation[operation.OperationID]
		if command == "" || seen[command] {
			continue
		}
		seen[command] = true
		switch {
		case operation.LiveTrading:
			live = append(live, command)
		case isOrderCommand(command):
			order = append(order, command)
		case isAccountCommand(command):
			account = append(account, command)
		default:
			regular = append(regular, command)
		}
	}
	return regular, account, order, live
}

func isAccountCommand(command string) bool {
	switch command {
	case "accounts", "holdings", "buying-power", "sellable-quantity", "commissions":
		return true
	default:
		return false
	}
}

func isOrderCommand(command string) bool {
	switch command {
	case "orders", "order":
		return true
	default:
		return false
	}
}

func isLiveTradingCommand(command string) bool {
	switch command {
	case "order-create", "order-modify", "order-cancel":
		return true
	default:
		return false
	}
}

func isSettingsCommand(command string) bool {
	switch command {
	case "features", "enable", "disable":
		return true
	default:
		return false
	}
}

func joinQuotedCommands(commands []string) string {
	quoted := make([]string, 0, len(commands))
	for _, command := range commands {
		quoted = append(quoted, "`"+command+"`")
	}
	return strings.Join(quoted, " and ")
}
