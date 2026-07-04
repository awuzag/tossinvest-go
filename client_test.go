package tossinvest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type seenRequest struct {
	method  string
	path    string
	query   string
	account string
	body    map[string]any
}

func TestTokenIssuesAndStoresAccessToken(t *testing.T) {
	var sawForm bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		sawForm = r.Form.Get("grant_type") == "client_credentials" &&
			r.Form.Get("client_id") == "client" &&
			r.Form.Get("client_secret") == "secret"
		_, _ = w.Write([]byte(`{"access_token":"issued","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL), WithClientID("client"), WithClientSecret("secret"))
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !sawForm {
		t.Fatal("token request did not send expected form")
	}
	if token.AccessToken != "issued" || client.currentAccessToken() != "issued" {
		t.Fatalf("unexpected token: %#v", token)
	}
}

func TestAccountAndOrderAPIsRequireExplicitEnablement(t *testing.T) {
	client, err := New(WithBaseURL("https://example.test"), WithAccessToken("token"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Accounts(context.Background()); !featureDisabled(err, "account APIs") {
		t.Fatalf("expected account API gate, got %T %[1]v", err)
	}
	if _, err := client.Orders(context.Background(), OrdersRequest{AccountSeq: "7"}); !featureDisabled(err, "account APIs") {
		t.Fatalf("expected account API gate before order API gate, got %T %[1]v", err)
	}

	accountClient, err := New(WithBaseURL("https://example.test"), WithAccessToken("token"), WithAccountAPIsEnabled())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accountClient.Orders(context.Background(), OrdersRequest{AccountSeq: "7"}); !featureDisabled(err, "order APIs") {
		t.Fatalf("expected order API gate, got %T %[1]v", err)
	}

	orderClient, err := New(WithBaseURL("https://example.test"), WithAccessToken("token"), WithAccountAPIsEnabled(), WithOrderAPIsEnabled())
	if err != nil {
		t.Fatal(err)
	}
	_, err = orderClient.CreateOrder(context.Background(), "7", OrderCreateRequest{Symbol: "005930", Side: "BUY", OrderType: "LIMIT", Quantity: "1", Price: "70000"})
	if !featureDisabled(err, "live trading") {
		t.Fatalf("expected live trading gate, got %T %[1]v", err)
	}
}

func TestAllOperationsUseExpectedHTTPContracts(t *testing.T) {
	var seen []seenRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record := seenRequest{
			method:  r.Method,
			path:    r.URL.Path,
			query:   r.URL.RawQuery,
			account: r.Header.Get("X-Tossinvest-Account"),
		}
		if r.Body != nil && r.Header.Get("content-type") == "application/json" {
			_ = json.NewDecoder(r.Body).Decode(&record.body)
		}
		seen = append(seen, record)
		if got := r.Header.Get("authorization"); got != "Bearer token" {
			t.Fatalf("%s missing bearer token: %q", r.URL.Path, got)
		}

		w.Header().Set("content-type", "application/json")
		switch r.URL.Path {
		case "/api/v1/prices", "/api/v1/stocks", "/api/v1/stocks/005930/warnings", "/api/v1/accounts", "/api/v1/trades", "/api/v1/commissions":
			_, _ = w.Write([]byte(`{"result":[]}`))
		case "/api/v1/orders/ord-1/modify", "/api/v1/orders/ord-1/cancel":
			_, _ = w.Write([]byte(`{"result":{"orderId":"ord-2"}}`))
		case "/api/v1/orders":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"result":{"orderId":"ord-1","clientOrderId":"client-1"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":{"orders":[],"nextCursor":null,"hasNext":false}}`))
		default:
			_, _ = w.Write([]byte(`{"result":{}}`))
		}
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithAccessToken("token"),
		WithAccountAPIsEnabled(),
		WithOrderAPIsEnabled(),
		WithLiveTradingEnabled(),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cases := []struct {
		name string
		run  func() error
	}{
		{"orderbook", func() error { _, err := client.Orderbook(ctx, "005930"); return err }},
		{"prices", func() error { _, err := client.Prices(ctx, []string{"005930", "AAPL"}); return err }},
		{"trades", func() error { _, err := client.Trades(ctx, "005930", 10); return err }},
		{"price limit", func() error { _, err := client.PriceLimit(ctx, "005930"); return err }},
		{"candles", func() error {
			adjusted := false
			_, err := client.Candles(ctx, CandlesRequest{Symbol: "005930", Interval: "1d", Count: 20, Adjusted: &adjusted})
			return err
		}},
		{"stocks", func() error { _, err := client.Stocks(ctx, []string{"005930"}); return err }},
		{"warnings", func() error { _, err := client.StockWarnings(ctx, "005930"); return err }},
		{"exchange rate", func() error {
			_, err := client.ExchangeRate(ctx, ExchangeRateRequest{BaseCurrency: CurrencyUSD, QuoteCurrency: CurrencyKRW})
			return err
		}},
		{"kr calendar", func() error { _, err := client.KrMarketCalendar(ctx, "2026-07-04"); return err }},
		{"us calendar", func() error { _, err := client.UsMarketCalendar(ctx, "2026-07-04"); return err }},
		{"accounts", func() error { _, err := client.Accounts(ctx); return err }},
		{"holdings", func() error { _, err := client.Holdings(ctx, "7", "005930"); return err }},
		{"orders", func() error {
			_, err := client.Orders(ctx, OrdersRequest{AccountSeq: "7", Status: "OPEN", Limit: 20})
			return err
		}},
		{"create order", func() error {
			_, err := client.CreateOrder(ctx, "7", OrderCreateRequest{ClientOrderID: "client-1", Symbol: "005930", Side: "BUY", OrderType: "LIMIT", Quantity: "1", Price: "70000"})
			return err
		}},
		{"order", func() error { _, err := client.Order(ctx, "7", "ord-1"); return err }},
		{"modify order", func() error {
			_, err := client.ModifyOrder(ctx, "7", "ord-1", OrderModifyRequest{OrderType: "LIMIT", Quantity: "1", Price: "71000"})
			return err
		}},
		{"cancel order", func() error { _, err := client.CancelOrder(ctx, "7", "ord-1"); return err }},
		{"buying power", func() error { _, err := client.BuyingPower(ctx, "7", CurrencyKRW); return err }},
		{"sellable quantity", func() error { _, err := client.SellableQuantity(ctx, "7", "005930"); return err }},
		{"commissions", func() error { _, err := client.Commissions(ctx, "7"); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err != nil {
				t.Fatal(err)
			}
		})
	}

	requireSeen(t, seen, http.MethodGet, "/api/v1/orderbook", "symbol=005930", "")
	requireSeen(t, seen, http.MethodGet, "/api/v1/holdings", "symbol=005930", "7")
	requireSeen(t, seen, http.MethodPost, "/api/v1/orders", "", "7")
	requireSeen(t, seen, http.MethodPost, "/api/v1/orders/ord-1/modify", "", "7")
	requireSeen(t, seen, http.MethodPost, "/api/v1/orders/ord-1/cancel", "", "7")
}

func featureDisabled(err error, feature string) bool {
	var disabled *FeatureDisabledError
	return errors.As(err, &disabled) && disabled.Feature == feature
}

func TestAPIAndOAuthErrors(t *testing.T) {
	t.Run("api error envelope", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"requestId":"req-1","code":"rate-limit-exceeded","message":"slow down","data":{"retryAfterSeconds":1}}}`))
		}))
		defer server.Close()
		client, err := New(WithBaseURL(server.URL), WithAccessToken("token"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Prices(context.Background(), []string{"005930"})
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected APIError, got %T %[1]v", err)
		}
		if apiErr.Code != "rate-limit-exceeded" || apiErr.RequestID != "req-1" {
			t.Fatalf("unexpected api error: %#v", apiErr)
		}
	})

	t.Run("oauth error envelope", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"bad client"}`))
		}))
		defer server.Close()
		client, err := New(WithBaseURL(server.URL), WithClientID("client"), WithClientSecret("secret"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Token(context.Background())
		var oauthErr *OAuthError
		if !errors.As(err, &oauthErr) {
			t.Fatalf("expected OAuthError, got %T %[1]v", err)
		}
		if oauthErr.ErrorCode != "invalid_client" {
			t.Fatalf("unexpected oauth error: %#v", oauthErr)
		}
	})
}

func TestHelpers(t *testing.T) {
	if got := bearer("abc"); got != "Bearer abc" {
		t.Fatalf("unexpected bearer: %q", got)
	}
	if got := bearer("Bearer abc"); got != "Bearer abc" {
		t.Fatalf("unexpected bearer passthrough: %q", got)
	}
	if got := SplitSymbols("005930, AAPL,,"); len(got) != 2 || got[1] != "AAPL" {
		t.Fatalf("unexpected symbols: %#v", got)
	}
}

func requireSeen(t *testing.T, seen []seenRequest, method string, path string, queryContains string, account string) {
	t.Helper()
	for _, item := range seen {
		if item.method == method && item.path == path && (queryContains == "" || strings.Contains(item.query, queryContains)) && item.account == account {
			return
		}
	}
	t.Fatalf("missing request method=%s path=%s query=%s account=%s in %#v", method, path, queryContains, account, seen)
}
