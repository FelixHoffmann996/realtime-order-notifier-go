#!/bin/sh
set -eu

curl --fail-with-body -sS -X POST http://localhost:8080/order-updates \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ord_42","account_id":"acct_7","status":"fulfillment_started"}'
printf '\n'

curl --fail-with-body -sS -X POST http://localhost:8080/realtime-token \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"browser_9","account_id":"acct_7"}'
printf '\n'
