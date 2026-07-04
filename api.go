package tossinvest

import (
	"context"
	"strings"

	tossapi "github.com/awuzag/tossinvest-go/internal/generated/tossapi"
)

func (c *Client) Orderbook(ctx context.Context, symbol string) (OrderbookResponse, error) {
	return tossapi.GetOrderbook(ctx, c, tossapi.GetOrderbookRequest{Symbol: symbol})
}

func (c *Client) Prices(ctx context.Context, symbols []string) ([]PriceResponse, error) {
	return tossapi.GetPrices(ctx, c, tossapi.GetPricesRequest{Symbols: comma(symbols)})
}

func (c *Client) Trades(ctx context.Context, symbol string, count int) ([]Trade, error) {
	return tossapi.GetTrades(ctx, c, tossapi.GetTradesRequest{Symbol: symbol, Count: count})
}

func (c *Client) PriceLimit(ctx context.Context, symbol string) (PriceLimitResponse, error) {
	return tossapi.GetPriceLimit(ctx, c, tossapi.GetPriceLimitRequest{Symbol: symbol})
}

func (c *Client) Candles(ctx context.Context, request CandlesRequest) (CandlePageResponse, error) {
	return tossapi.GetCandles(ctx, c, tossapi.GetCandlesRequest{
		Symbol:   request.Symbol,
		Interval: request.Interval,
		Count:    request.Count,
		Before:   request.Before,
		Adjusted: request.Adjusted,
	})
}

func (c *Client) Stocks(ctx context.Context, symbols []string) ([]StockInfo, error) {
	return tossapi.GetStocks(ctx, c, tossapi.GetStocksRequest{Symbols: comma(symbols)})
}

func (c *Client) StockWarnings(ctx context.Context, symbol string) ([]StockWarning, error) {
	return tossapi.GetStockWarnings(ctx, c, tossapi.GetStockWarningsRequest{Symbol: symbol})
}

func (c *Client) ExchangeRate(ctx context.Context, request ExchangeRateRequest) (ExchangeRateResponse, error) {
	return tossapi.GetExchangeRate(ctx, c, tossapi.GetExchangeRateRequest{
		DateTime:      request.DateTime,
		BaseCurrency:  request.BaseCurrency,
		QuoteCurrency: request.QuoteCurrency,
	})
}

func (c *Client) KrMarketCalendar(ctx context.Context, date string) (KrMarketCalendarResponse, error) {
	return tossapi.GetKRMarketCalendar(ctx, c, tossapi.GetKRMarketCalendarRequest{Date: date})
}

func (c *Client) UsMarketCalendar(ctx context.Context, date string) (UsMarketCalendarResponse, error) {
	return tossapi.GetUSMarketCalendar(ctx, c, tossapi.GetUSMarketCalendarRequest{Date: date})
}

func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	return tossapi.GetAccounts(ctx, c, tossapi.GetAccountsRequest{})
}

func (c *Client) Holdings(ctx context.Context, accountSeq string, symbol string) (HoldingsOverview, error) {
	return tossapi.GetHoldings(ctx, c, tossapi.GetHoldingsRequest{AccountSeq: accountSeq, Symbol: symbol})
}

func (c *Client) Orders(ctx context.Context, request OrdersRequest) (PaginatedOrderResponse, error) {
	return tossapi.GetOrders(ctx, c, tossapi.GetOrdersRequest{
		AccountSeq: request.AccountSeq,
		Status:     request.Status,
		Symbol:     request.Symbol,
		From:       request.From,
		To:         request.To,
		Cursor:     request.Cursor,
		Limit:      request.Limit,
	})
}

func (c *Client) CreateOrder(ctx context.Context, accountSeq string, request OrderCreateRequest) (OrderResponse, error) {
	return tossapi.CreateOrder(ctx, c, tossapi.CreateOrderRequest{
		AccountSeq: accountSeq,
		Body: tossapi.OrderCreateRequest{
			ClientOrderID:         request.ClientOrderID,
			Symbol:                request.Symbol,
			Side:                  request.Side,
			OrderType:             request.OrderType,
			TimeInForce:           request.TimeInForce,
			Quantity:              request.Quantity,
			Price:                 request.Price,
			OrderAmount:           request.OrderAmount,
			ConfirmHighValueOrder: request.ConfirmHighValue,
		},
	})
}

func (c *Client) Order(ctx context.Context, accountSeq string, orderID string) (Order, error) {
	return tossapi.GetOrder(ctx, c, tossapi.GetOrderRequest{AccountSeq: accountSeq, OrderID: orderID})
}

func (c *Client) ModifyOrder(ctx context.Context, accountSeq string, orderID string, request OrderModifyRequest) (OrderOperationResponse, error) {
	return tossapi.ModifyOrder(ctx, c, tossapi.ModifyOrderRequest{
		AccountSeq: accountSeq,
		OrderID:    orderID,
		Body: tossapi.OrderModifyRequest{
			OrderType:             request.OrderType,
			Quantity:              request.Quantity,
			Price:                 request.Price,
			ConfirmHighValueOrder: request.ConfirmHighValue,
		},
	})
}

func (c *Client) CancelOrder(ctx context.Context, accountSeq string, orderID string) (OrderOperationResponse, error) {
	return tossapi.CancelOrder(ctx, c, tossapi.CancelOrderRequest{AccountSeq: accountSeq, OrderID: orderID})
}

func (c *Client) BuyingPower(ctx context.Context, accountSeq string, currency Currency) (BuyingPowerResponse, error) {
	return tossapi.GetBuyingPower(ctx, c, tossapi.GetBuyingPowerRequest{AccountSeq: accountSeq, Currency: currency})
}

func (c *Client) SellableQuantity(ctx context.Context, accountSeq string, symbol string) (SellableQuantityResponse, error) {
	return tossapi.GetSellableQuantity(ctx, c, tossapi.GetSellableQuantityRequest{AccountSeq: accountSeq, Symbol: symbol})
}

func (c *Client) Commissions(ctx context.Context, accountSeq string) ([]Commission, error) {
	return tossapi.GetCommissions(ctx, c, tossapi.GetCommissionsRequest{AccountSeq: accountSeq})
}

func SplitSymbols(symbols string) []string {
	parts := strings.Split(symbols, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
