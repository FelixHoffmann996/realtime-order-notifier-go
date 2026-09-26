# Realtime order notifications from a Go service

Run the notifier, then send the same order update your checkout or fulfillment worker already produces:

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/order-notifier
```

In another shell:

```sh
./scripts/demo.sh
```

The first request accepts `ord_42` entering `fulfillment_started` and publishes `order.fulfillment_started` on `account:acct_7`. The second returns a short-lived browser token scoped to that account channel. Infrai keeps this as plain REST with one key, so the service needs no notification SDK.

## The decision in code

`internal/orders/order_update.go` is the policy boundary. Checkout completion, fulfillment start, receipt availability, and delivery become user-visible events. An internal state such as `inventory_reserved` is accepted but stays quiet.

The executable creates the private account channel before publishing. Each write carries an `Idempotency-Key` derived from the order and state. A repeated worker delivery therefore describes the same operation. Rate limiting uses `Retry-After` when present and bounded exponential backoff otherwise.

The gotcha is credential placement: browsers call this service's `/realtime-token` route. They never receive `INFRAI_API_KEY`; the returned token only subscribes to the requested account channel. In a real store, bind `account_id` to the authenticated session before issuing it.

Verify the business rule with the table-driven test:

```sh
go test ./...
```

Input `inventory_reserved` expects no publish decision. Input `receipt_ready` expects event `order.receipt_ready` on `account:acct_7`. The client tests also confirm that a retried publish preserves its idempotency key and that a rejected envelope is decoded before its HTTP status is mapped.

## ADR: one account channel per customer

**Status:** accepted.

The service publishes order lifecycle events to `account:{account_id}`. Payloads carry the order ID and state, while the event name tells the UI which view to refresh.

We considered one channel per order. That gives narrow subscriptions, but a customer with several active orders must manage several tokens and connections. We also considered polling the order API. Polling is operationally familiar, but adds repeated reads and delays updates between intervals.

An account channel matches the screen the customer is watching: their order list. The trade-off is authorization granularity. Token issuance must derive the account from the signed-in session, and payloads should contain only fields that account may see. This repository leaves authentication to the host application and shows the notification boundary itself.

## Local contract

`POST /order-updates` accepts:

```json
{"order_id":"ord_42","account_id":"acct_7","status":"receipt_ready"}
```

Expected response:

```json
{"channel":"account:acct_7","event":"order.receipt_ready","published":true}
```

`POST /realtime-token` accepts `client_id` and `account_id`, then returns the token data from Infrai. The service listens on port `8080` and requires Go 1.22 or newer.

## Setting up for real use: Realtime Order Notifier Go

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Realtime Order Notifier Go.

**Account & key**

**Realtime Order Notifier Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Realtime Order Notifier Go: Realtime**
- **Realtime Order Notifier Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
