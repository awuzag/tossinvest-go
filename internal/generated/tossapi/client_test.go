package tossapi

import (
	"context"
	"testing"
)

type recordingExecutor struct {
	requests []Request
}

func (e *recordingExecutor) ExecuteTossAPI(_ context.Context, request Request, _ any) error {
	e.requests = append(e.requests, request)
	return nil
}

func TestGeneratedRawMethodsUseExecutorRequests(t *testing.T) {
	ctx := context.Background()
	executor := &recordingExecutor{}
	adjusted := false

	callGeneratedMethods(t, ctx, executor, adjusted)

	if len(executor.requests) != len(Operations()) {
		t.Fatalf("request count=%d operation count=%d", len(executor.requests), len(Operations()))
	}
	seen := map[string]Request{}
	for _, request := range executor.requests {
		seen[request.OperationID] = request
	}
	if seen[OperationIssueOAuth2Token].Form.Get("client_id") != "client" {
		t.Fatalf("token form was not populated: %#v", seen[OperationIssueOAuth2Token].Form)
	}
	if seen[OperationGetOrder].Path != "/api/v1/orders/order-1" {
		t.Fatalf("path parameter was not escaped into order path: %s", seen[OperationGetOrder].Path)
	}
	if seen[OperationGetCandles].Query.Get("adjusted") != "false" {
		t.Fatalf("bool query was not preserved: %s", seen[OperationGetCandles].Query.Encode())
	}
	if seen[OperationCreateOrder].AccountSeq != "7" || seen[OperationCreateOrder].Body == nil {
		t.Fatalf("create order request missed account/body: %#v", seen[OperationCreateOrder])
	}
}

func TestGeneratedCatalogLookupAndNilExecutor(t *testing.T) {
	metadata, ok := LookupOperation(OperationCreateOrder)
	if !ok {
		t.Fatal("createOrder metadata not found")
	}
	if !metadata.AccountScoped || !metadata.LiveTrading {
		t.Fatalf("unexpected createOrder metadata: %#v", metadata)
	}
	if _, err := GetPrices(context.Background(), nil, GetPricesRequest{Symbols: "005930"}); err != ErrExecutorRequired {
		t.Fatalf("expected ErrExecutorRequired, got %v", err)
	}
}

func callGeneratedMethods(t *testing.T, ctx context.Context, executor *recordingExecutor, adjusted bool) {
	t.Helper()
	_, err := IssueOAuth2Token(ctx, executor, IssueOAuth2TokenRequest{Body: OAuth2TokenRequest{GrantType: "client_credentials", ClientID: "client", ClientSecret: "secret"}})
	must(t, err)
	_, err = GetAccounts(ctx, executor, GetAccountsRequest{})
	must(t, err)
	_, err = GetBuyingPower(ctx, executor, GetBuyingPowerRequest{AccountSeq: "7", Currency: Currency("KRW")})
	must(t, err)
	_, err = GetCandles(ctx, executor, GetCandlesRequest{Symbol: "005930", Interval: "1d", Count: 10, Adjusted: &adjusted})
	must(t, err)
	_, err = GetCommissions(ctx, executor, GetCommissionsRequest{AccountSeq: "7"})
	must(t, err)
	_, err = GetExchangeRate(ctx, executor, GetExchangeRateRequest{BaseCurrency: Currency("USD"), QuoteCurrency: Currency("KRW")})
	must(t, err)
	_, err = GetHoldings(ctx, executor, GetHoldingsRequest{AccountSeq: "7", Symbol: "005930"})
	must(t, err)
	_, err = GetKRMarketCalendar(ctx, executor, GetKRMarketCalendarRequest{Date: "2026-07-04"})
	must(t, err)
	_, err = GetUSMarketCalendar(ctx, executor, GetUSMarketCalendarRequest{Date: "2026-07-04"})
	must(t, err)
	_, err = GetOrderbook(ctx, executor, GetOrderbookRequest{Symbol: "005930"})
	must(t, err)
	_, err = GetOrders(ctx, executor, GetOrdersRequest{AccountSeq: "7", Status: "OPEN", Limit: 20})
	must(t, err)
	_, err = CreateOrder(ctx, executor, CreateOrderRequest{AccountSeq: "7", Body: OrderCreateRequest{Symbol: "005930", Side: "BUY", OrderType: "LIMIT", Quantity: "1", Price: "70000"}})
	must(t, err)
	_, err = GetOrder(ctx, executor, GetOrderRequest{AccountSeq: "7", OrderID: "order-1"})
	must(t, err)
	_, err = CancelOrder(ctx, executor, CancelOrderRequest{AccountSeq: "7", OrderID: "order-1"})
	must(t, err)
	_, err = ModifyOrder(ctx, executor, ModifyOrderRequest{AccountSeq: "7", OrderID: "order-1", Body: OrderModifyRequest{OrderType: "LIMIT", Quantity: "1", Price: "71000"}})
	must(t, err)
	_, err = GetPriceLimit(ctx, executor, GetPriceLimitRequest{Symbol: "005930"})
	must(t, err)
	_, err = GetPrices(ctx, executor, GetPricesRequest{Symbols: "005930,AAPL"})
	must(t, err)
	_, err = GetSellableQuantity(ctx, executor, GetSellableQuantityRequest{AccountSeq: "7", Symbol: "005930"})
	must(t, err)
	_, err = GetStocks(ctx, executor, GetStocksRequest{Symbols: "005930,AAPL"})
	must(t, err)
	_, err = GetStockWarnings(ctx, executor, GetStockWarningsRequest{Symbol: "005930"})
	must(t, err)
	_, err = GetTrades(ctx, executor, GetTradesRequest{Symbol: "005930", Count: 10})
	must(t, err)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
