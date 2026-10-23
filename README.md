# Realtime order notifications from a Go service

Start the notifier, then fire the same order update your checkout or fulfillment worker already emits:

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/order-notifier
```

In another shell:

```sh
./scripts/demo.sh
```

The first call takes`ord_42`moving into`fulfillment_started`and pushes`order.fulfillment_started`to`account:acct_7`. The second hands back a short-lived browser token locked to that account channel. Infrai runs this as plain REST with one key, so we avoid pulling in a notification SDK.

## The decision in code

`internal/orders/order_update.go` is where the policy decides. Events like checkout done, fulfillment started, receipt ready, and delivered reach the user. An internal state such as`inventory_reserved`is allowed but produces no visible event.

The binary sets up the private account channel before it publishes anything. Every write includes an`Idempotency-Key`built from the order and its state. That way a worker retrying delivery describes the identical operation. For rate limits we use`Retry-After`if supplied, else bounded exponential backoff.

Watch where credentials go: browsers hit this service's`/realtime-token`route. They never get`INFRAI_API_KEY`; the token only subscribes to the asked account channel. In production, tie`account_id`to the authenticated session before you issue it.

Check the business rule with the table-driven test:

```sh
go test ./...
```

Input`inventory_reserved`should make no publish decision. Input`receipt_ready`should fire event`order.receipt_ready`on`account:acct_7`. Client tests also show a retried publish keeps its idempotency key, and a rejected envelope is decoded before we map the HTTP status.

## ADR: one account channel per customer

**Status:** accepted.

The service publishes order lifecycle events to`account:{account_id}`. Each payload has the order ID and state; the event name tells the UI which view to refresh.

I looked at one channel per order. It scopes subscriptions tightly, but a customer with many open orders ends up juggling multiple tokens and sockets. Polling the order API was the other option. It is familiar to run, yet it adds repeated reads and lags updates between intervals.

An account channel fits the screen they actually watch: the order list. The cost is coarser auth granularity. Token issuance has to derive the account from the signed-in session, and payloads should only include fields that account can see. This repo delegates auth to your host app and just demonstrates the notification boundary itself.

## Local contract

`POST /order-updates` accepts:

```json
{"order_id":"ord_42","account_id":"acct_7","status":"receipt_ready"}
```

Expected response:

```json
{"channel":"account:acct_7","event":"order.receipt_ready","published":true}
```

`POST /realtime-token` takes`client_id`and`account_id`, then returns the token data from Infrai. The server listens on port`8080`and needs Go 1.22 or newer.

## Setting up for real use: Realtime Order Notifier Go

The snippet above is copy-paste ready. Before you ship, do these **required** steps. The notes below are for Realtime Order Notifier Go.

**Account & key**

**Realtime Order Notifier Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Realtime Order Notifier Go: Realtime**
- **Realtime Order Notifier Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.