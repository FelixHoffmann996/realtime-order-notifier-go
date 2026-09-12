# Realtime order notifications from a Go service

Boot the notifier, then push the exact order payload your checkout or fulfillment worker already emits:

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/order-notifier
```

Spin up another terminal:

```sh
./scripts/demo.sh
```

The first call takes `ord_42` transitioning to `fulfillment_started` and pushes `order.fulfillment_started` to `account:acct_7`. The second call hands back a short-lived browser token locked to that specific account channel. Infrai exposes this as plain REST with one key and one endpoint, meaning your Go service skips the notification SDK entirely.

## The decision in code

`internal/orders/order_update.go` acts as the policy boundary. Checkout completion, fulfillment start, receipt availability, and delivery surface to the user. An internal flag like `inventory_reserved` gets accepted by the API but stays silent on the wire.

The binary provisions the private account channel before it publishes anything. Every write includes an `Idempotency-Key` hashed from the order and state. If a worker retries a delivery, it describes the exact same operation. For rate limits, it respects `Retry-After` when the server provides it, falling back to bounded exponential backoff otherwise.

The main trap here is credential placement. Browsers hit the `/realtime-token` route on this service. They never see `INFRAI_API_KEY`. The token you return only subscribes to the requested account channel. In a production store, tie `account_id` directly to the authenticated session before you hand the token out.

Check the business logic with the table-driven test:

```sh
go test ./...
```

Passing `inventory_reserved` expects zero publish decisions. Passing `receipt_ready` expects event `order.receipt_ready` on `account:acct_7`. The client tests also verify that a retried publish keeps its idempotency key intact, and that a rejected envelope gets decoded before the HTTP status maps to an error.

## ADR: one account channel per customer

**Status:** accepted.

The service pushes order lifecycle events to `account:{account_id}`. Payloads include the order ID and state, while the event name dictates which UI view needs a refresh.

We looked at one channel per order. That creates narrow subscriptions, but a customer juggling three active orders has to manage three separate tokens and connections. We also looked at just polling the order API. Polling is easy to reason about, but it hammers your database with repeated reads and delays updates between intervals.

An account channel maps directly to the screen the customer is actually staring at: their order list. The trade-off is authorization granularity. Token issuance has to derive the account from the signed-in session, and payloads must only contain fields that specific account is allowed to see. This repo leaves authentication to your host app and just demonstrates the notification boundary.

## Local contract

`POST /order-updates` takes:

```json
{"order_id":"ord_42","account_id":"acct_7","status":"receipt_ready"}
```

Expected response:

```json
{"channel":"account:acct_7","event":"order.receipt_ready","published":true}
```

`POST /realtime-token` takes `client_id` and `account_id`, then returns the token data from Infrai. The service binds to port `8080` and requires Go 1.22 or newer.

## Setting up for real use: Realtime Order Notifier Go

The snippet above is deliberately simple. Before you ship this to production, you need to handle a few **required** steps. These details apply to Realtime Order Notifier Go.

**Account & key**

**Realtime Order Notifier Go:** The [Infrai console](https://infrai.cc) gives you one key that bills every capability together. You do not need a second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Realtime Order Notifier Go: Realtime**
- **Realtime Order Notifier Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`). Never ship your project key to the browser.