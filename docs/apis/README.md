# Toss Invest OpenAPI Inventory

This document is the human-readable API inventory. The canonical OpenAPI contract is kept outside docs at `contracts/tossinvest/openapi.json` because it is build-critical input for generated SDK/CLI code.

Source snapshot:

- workspace catalog: `workspace/docs/providers/tossinvest/source-catalog.md`
- workspace guide: `workspace/docs/providers/tossinvest/guide.md`
- upstream OpenAPI URL: `https://openapi.tossinvest.com/openapi-docs/latest/openapi.json`

For the code generation and middle-layer plan, see `docs/architecture/contract-codegen.md`.

## API Coverage

| Group | SDK method | HTTP API | CLI command |
| --- | --- | --- | --- |
| Auth | `Token` | `POST /oauth2/token` | `token` |
| Market Data | `Orderbook` | `GET /api/v1/orderbook` | `orderbook` |
| Market Data | `Prices` | `GET /api/v1/prices` | `prices` |
| Market Data | `Trades` | `GET /api/v1/trades` | `trades` |
| Market Data | `PriceLimit` | `GET /api/v1/price-limits` | `price-limits` |
| Market Data | `Candles` | `GET /api/v1/candles` | `candles` |
| Stock Info | `Stocks` | `GET /api/v1/stocks` | `stocks` |
| Stock Info | `StockWarnings` | `GET /api/v1/stocks/{symbol}/warnings` | `stock-warnings` |
| Market Info | `ExchangeRate` | `GET /api/v1/exchange-rate` | `exchange-rate` |
| Market Info | `KrMarketCalendar` | `GET /api/v1/market-calendar/KR` | `market-calendar --country KR` |
| Market Info | `UsMarketCalendar` | `GET /api/v1/market-calendar/US` | `market-calendar --country US` |
| Account | `Accounts` | `GET /api/v1/accounts` | `accounts` after `enable account` |
| Asset | `Holdings` | `GET /api/v1/holdings` | `holdings` after `enable account` |
| Order History | `Orders` | `GET /api/v1/orders` | `orders` after `enable account` and `enable orders` |
| Order History | `Order` | `GET /api/v1/orders/{orderId}` | `order` after `enable account` and `enable orders` |
| Order | `CreateOrder` | `POST /api/v1/orders` | `order-create` after `enable account`, `enable orders`, and `enable live-trading confirm-live-orders` |
| Order | `ModifyOrder` | `POST /api/v1/orders/{orderId}/modify` | `order-modify` after `enable account`, `enable orders`, and `enable live-trading confirm-live-orders` |
| Order | `CancelOrder` | `POST /api/v1/orders/{orderId}/cancel` | `order-cancel` after `enable account`, `enable orders`, and `enable live-trading confirm-live-orders` |
| Order Info | `BuyingPower` | `GET /api/v1/buying-power` | `buying-power` after `enable account` |
| Order Info | `SellableQuantity` | `GET /api/v1/sellable-quantity` | `sellable-quantity` after `enable account` |
| Order Info | `Commissions` | `GET /api/v1/commissions` | `commissions` after `enable account` |

## Data Handling Rules

- Decimal values stay as strings.
- Unknown enum values must be tolerated by callers.
- Account-scoped APIs send `X-Tossinvest-Account`.
- Account and order APIs are disabled by default and require explicit SDK/CLI opt-in.
- CLI opt-ins are stored in local config; live-trading activation expires after 15 minutes.
- OAuth token issuance uses form encoding and does not use the common API envelope.
- Ordinary API success responses use `{ "result": ... }`.
- Ordinary API error responses use `{ "error": { "requestId", "code", "message", "data" } }`.
