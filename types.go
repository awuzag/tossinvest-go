package tossinvest

import (
	"encoding/json"

	tossapi "github.com/awuzag/tossinvest-go/internal/generated/tossapi"
)

type Currency = tossapi.Currency
type MarketCountry = tossapi.MarketCountry

const (
	CurrencyKRW Currency = "KRW"
	CurrencyUSD Currency = "USD"

	MarketCountryKR MarketCountry = "KR"
	MarketCountryUS MarketCountry = "US"
)

type RawObject map[string]any

type Token = tossapi.OAuth2TokenResponse

type RateLimit struct {
	Limit      string
	Remaining  string
	Reset      string
	RetryAfter string
	RequestID  string
}

type Account = tossapi.Account

type OrderCreateRequest struct {
	ClientOrderID    string `json:"clientOrderId,omitempty"`
	Symbol           string `json:"symbol"`
	Side             string `json:"side"`
	OrderType        string `json:"orderType"`
	TimeInForce      string `json:"timeInForce,omitempty"`
	Quantity         string `json:"quantity,omitempty"`
	Price            string `json:"price,omitempty"`
	OrderAmount      string `json:"orderAmount,omitempty"`
	ConfirmHighValue bool   `json:"confirmHighValueOrder,omitempty"`
}

type OrderModifyRequest struct {
	OrderType        string `json:"orderType"`
	Quantity         string `json:"quantity,omitempty"`
	Price            string `json:"price,omitempty"`
	ConfirmHighValue bool   `json:"confirmHighValueOrder,omitempty"`
}

type OrdersRequest struct {
	AccountSeq string
	Status     string
	Symbol     string
	From       string
	To         string
	Cursor     string
	Limit      int
}

type CandlesRequest struct {
	Symbol   string
	Interval string
	Count    int
	Before   string
	Adjusted *bool
}

type ExchangeRateRequest struct {
	DateTime      string
	BaseCurrency  Currency
	QuoteCurrency Currency
}

type Envelope[T any] struct {
	Result T `json:"result"`
}

type RawEnvelope struct {
	Result json.RawMessage `json:"result"`
}

type OrderbookResponse = tossapi.OrderbookResponse
type PriceResponse = tossapi.PriceResponse
type Trade = tossapi.Trade
type PriceLimitResponse = tossapi.PriceLimitResponse
type CandlePageResponse = tossapi.CandlePageResponse
type StockInfo = tossapi.StockInfo
type StockWarning = tossapi.StockWarning
type ExchangeRateResponse = tossapi.ExchangeRateResponse
type KrMarketCalendarResponse = tossapi.KRMarketCalendarResponse
type UsMarketCalendarResponse = tossapi.USMarketCalendarResponse
type HoldingsOverview = tossapi.HoldingsOverview
type OrderResponse = tossapi.OrderResponse
type OrderOperationResponse = tossapi.OrderOperationResponse
type PaginatedOrderResponse = tossapi.PaginatedOrderResponse
type Order = tossapi.Order
type BuyingPowerResponse = tossapi.BuyingPowerResponse
type SellableQuantityResponse = tossapi.SellableQuantityResponse
type Commission = tossapi.Commission
