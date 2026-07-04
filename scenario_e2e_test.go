package tossinvest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScenarioE2EMarketDataDiscovery(t *testing.T) {
	events := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		events = append(events, r.Method+" "+r.URL.Path)
		w.Header().Set("content-type", "application/json")
		switch r.URL.Path {
		case "/oauth2/token":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("grant_type") != "client_credentials" {
				t.Fatalf("unexpected grant: %s", r.Form.Get("grant_type"))
			}
			_, _ = w.Write([]byte(`{"access_token":"scenario-token","token_type":"Bearer","expires_in":3600}`))
		case "/api/v1/stocks":
			requireBearer(t, r)
			requireQuery(t, r, "symbols", "005930,AAPL")
			_, _ = w.Write([]byte(`{"result":[{"symbol":"005930","name":"삼성전자","currency":"KRW"},{"symbol":"AAPL","name":"Apple","currency":"USD"}]}`))
		case "/api/v1/prices":
			requireBearer(t, r)
			requireQuery(t, r, "symbols", "005930,AAPL")
			_, _ = w.Write([]byte(`{"result":[{"symbol":"005930","lastPrice":"70000","currency":"KRW"},{"symbol":"AAPL","lastPrice":"185.5","currency":"USD"}]}`))
		case "/api/v1/orderbook":
			requireBearer(t, r)
			requireQuery(t, r, "symbol", "005930")
			_, _ = w.Write([]byte(`{"result":{"timestamp":"2026-07-04T09:00:00+09:00","currency":"KRW","asks":[{"price":"70100","volume":"10"}],"bids":[{"price":"70000","volume":"12"}]}}`))
		case "/api/v1/trades":
			requireBearer(t, r)
			requireQuery(t, r, "symbol", "005930")
			requireQuery(t, r, "count", "2")
			_, _ = w.Write([]byte(`{"result":[{"price":"70000","volume":"1","timestamp":"2026-07-04T09:00:01+09:00","currency":"KRW"}]}`))
		case "/api/v1/candles":
			requireBearer(t, r)
			requireQuery(t, r, "symbol", "005930")
			requireQuery(t, r, "interval", "1d")
			_, _ = w.Write([]byte(`{"result":{"candles":[{"timestamp":"2026-07-04T00:00:00+09:00","openPrice":"69000","highPrice":"71000","lowPrice":"68000","closePrice":"70000","volume":"1000","currency":"KRW"}],"nextBefore":null}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL), WithClientID("client"), WithClientSecret("secret"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if stocks, err := client.Stocks(ctx, []string{"005930", "AAPL"}); err != nil || len(stocks) != 2 {
		t.Fatalf("stocks len=%d err=%v", len(stocks), err)
	}
	if prices, err := client.Prices(ctx, []string{"005930", "AAPL"}); err != nil || len(prices) != 2 {
		t.Fatalf("prices len=%d err=%v", len(prices), err)
	}
	if _, err := client.Orderbook(ctx, "005930"); err != nil {
		t.Fatal(err)
	}
	if trades, err := client.Trades(ctx, "005930", 2); err != nil || len(trades) != 1 {
		t.Fatalf("trades len=%d err=%v", len(trades), err)
	}
	if _, err := client.Candles(ctx, CandlesRequest{Symbol: "005930", Interval: "1d", Count: 1}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(events, "|") != "POST /oauth2/token|GET /api/v1/stocks|GET /api/v1/prices|GET /api/v1/orderbook|GET /api/v1/trades|GET /api/v1/candles" {
		t.Fatalf("unexpected scenario events: %#v", events)
	}
}

func TestScenarioE2EAccountOrderLifecycle(t *testing.T) {
	seenAccountHeaders := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		switch r.URL.Path {
		case "/api/v1/accounts":
			requireBearer(t, r)
			_, _ = w.Write([]byte(`{"result":[{"accountNo":"12345678901","accountSeq":7,"accountType":"BROKERAGE"}]}`))
		case "/api/v1/holdings":
			requireBearer(t, r)
			requireAccount(t, r, "7")
			seenAccountHeaders++
			_, _ = w.Write([]byte(`{"result":{"totalPurchaseAmount":{"krw":"0"},"marketValue":{"krw":"0"},"profitLoss":{"krw":"0"},"dailyProfitLoss":{"krw":"0"},"items":[]}}`))
		case "/api/v1/buying-power":
			requireBearer(t, r)
			requireAccount(t, r, "7")
			seenAccountHeaders++
			requireQuery(t, r, "currency", "KRW")
			_, _ = w.Write([]byte(`{"result":{"currency":"KRW","cashBuyingPower":"1000000"}}`))
		case "/api/v1/sellable-quantity":
			requireBearer(t, r)
			requireAccount(t, r, "7")
			seenAccountHeaders++
			requireQuery(t, r, "symbol", "005930")
			_, _ = w.Write([]byte(`{"result":{"sellableQuantity":"10"}}`))
		case "/api/v1/commissions":
			requireBearer(t, r)
			requireAccount(t, r, "7")
			seenAccountHeaders++
			_, _ = w.Write([]byte(`{"result":[{"marketCountry":"KR","commissionRate":"0.015"}]}`))
		case "/api/v1/orders":
			requireBearer(t, r)
			requireAccount(t, r, "7")
			seenAccountHeaders++
			if r.Method == http.MethodGet {
				requireQuery(t, r, "status", "OPEN")
				_, _ = w.Write([]byte(`{"result":{"orders":[],"nextCursor":null,"hasNext":false}}`))
				return
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["clientOrderId"] != "scenario-1" || body["price"] != "70000" {
				t.Fatalf("unexpected create order body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"result":{"orderId":"ord-1","clientOrderId":"scenario-1"}}`))
		case "/api/v1/orders/ord-1":
			requireBearer(t, r)
			requireAccount(t, r, "7")
			seenAccountHeaders++
			_, _ = w.Write([]byte(`{"result":{"orderId":"ord-1","symbol":"005930","side":"BUY","orderType":"LIMIT","timeInForce":"DAY","status":"PENDING","quantity":"1","currency":"KRW","orderedAt":"2026-07-04T09:00:00+09:00","execution":{"filledQuantity":"0","averageFilledPrice":null,"filledAmount":null,"commission":null,"tax":null,"filledAt":null,"settlementDate":null}}}`))
		case "/api/v1/orders/ord-1/modify":
			requireBearer(t, r)
			requireAccount(t, r, "7")
			seenAccountHeaders++
			_, _ = w.Write([]byte(`{"result":{"orderId":"ord-2"}}`))
		case "/api/v1/orders/ord-1/cancel":
			requireBearer(t, r)
			requireAccount(t, r, "7")
			seenAccountHeaders++
			_, _ = w.Write([]byte(`{"result":{"orderId":"ord-3"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := New(
		WithBaseURL(server.URL),
		WithAccessToken("token"),
		WithAccountSeq("7"),
		WithAccountAPIsEnabled(),
		WithOrderAPIsEnabled(),
		WithLiveTradingEnabled(),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	accounts, err := client.Accounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].AccountSeq != 7 {
		t.Fatalf("unexpected accounts: %#v", accounts)
	}
	accountSeq := "7"
	if _, err := client.Holdings(ctx, accountSeq, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := client.BuyingPower(ctx, accountSeq, CurrencyKRW); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SellableQuantity(ctx, accountSeq, "005930"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Commissions(ctx, accountSeq); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Orders(ctx, OrdersRequest{AccountSeq: accountSeq, Status: "OPEN"}); err != nil {
		t.Fatal(err)
	}
	created, err := client.CreateOrder(ctx, accountSeq, OrderCreateRequest{ClientOrderID: "scenario-1", Symbol: "005930", Side: "BUY", OrderType: "LIMIT", Quantity: "1", Price: "70000"})
	if err != nil {
		t.Fatal(err)
	}
	if created.OrderID != "ord-1" {
		t.Fatalf("unexpected order create response: %#v", created)
	}
	if _, err := client.Order(ctx, accountSeq, "ord-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ModifyOrder(ctx, accountSeq, "ord-1", OrderModifyRequest{OrderType: "LIMIT", Quantity: "1", Price: "71000"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CancelOrder(ctx, accountSeq, "ord-1"); err != nil {
		t.Fatal(err)
	}
	if seenAccountHeaders != 9 {
		t.Fatalf("expected 9 account-scoped calls, got %d", seenAccountHeaders)
	}
}

func requireBearer(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Header.Get("authorization") != "Bearer token" && r.Header.Get("authorization") != "Bearer scenario-token" {
		t.Fatalf("missing bearer token for %s: %s", r.URL.Path, r.Header.Get("authorization"))
	}
}

func requireAccount(t *testing.T, r *http.Request, accountSeq string) {
	t.Helper()
	if got := r.Header.Get("X-Tossinvest-Account"); got != accountSeq {
		t.Fatalf("account header mismatch for %s: got %q want %q", r.URL.Path, got, accountSeq)
	}
}

func requireQuery(t *testing.T, r *http.Request, key string, value string) {
	t.Helper()
	if got := r.URL.Query().Get(key); got != value {
		t.Fatalf("query %s mismatch for %s: got %q want %q", key, r.URL.Path, got, value)
	}
}
