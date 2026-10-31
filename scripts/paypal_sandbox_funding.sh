#!/usr/bin/env bash

set -Eeuo pipefail

readonly PAYPAL_API_BASE="${PAYPAL_API_BASE:-https://api-m.sandbox.paypal.com}"

usage() {
  cat <<'EOF'
Test the PayPal Sandbox funding flow (Personal buyer -> Controlled Business account).

Usage:
  PAYPAL_CLIENT_ID=... PAYPAL_CLIENT_SECRET=... \
    scripts/paypal_sandbox_funding.sh run [AMOUNT] [CURRENCY]

  PAYPAL_CLIENT_ID=... PAYPAL_CLIENT_SECRET=... \
    scripts/paypal_sandbox_funding.sh create [AMOUNT] [CURRENCY]

  PAYPAL_CLIENT_ID=... PAYPAL_CLIENT_SECRET=... \
    scripts/paypal_sandbox_funding.sh capture ORDER_ID

Commands:
  run      Create an order, wait for buyer approval, and capture it (default).
  create   Create an order and print its approval URL without capturing it.
  capture  Capture an already-approved order.

Defaults:
  AMOUNT=10.00
  CURRENCY=USD

Optional environment variables:
  CONTROLLED_FUNDING_REFERENCE  Internal reference included as custom_id.
  PAYPAL_RETURN_URL             Buyer return URL (default: https://example.com/paypal/return).
  PAYPAL_CANCEL_URL             Buyer cancel URL (default: https://example.com/paypal/cancel).
  PAYPAL_API_BASE               Override only for compatible test environments.
EOF
}

die() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

request_id() {
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]'
  else
    printf 'controlled-%s-%s\n' "$(date +%s)" "$$"
  fi
}

paypal_error() {
  local response=$1
  jq -r '
    if type == "object" then
      "PayPal API error: " +
      ((.name // "UNKNOWN_ERROR") | tostring) +
      (if .message then " - " + (.message | tostring) else "" end) +
      (if .debug_id then " (debug_id: " + (.debug_id | tostring) + ")" else "" end)
    else
      "PayPal API returned an unexpected response"
    end
  ' <<<"$response" >&2 2>/dev/null || printf '%s\n' "$response" >&2
}

get_access_token() {
  local response token

  if ! response=$(curl --silent --show-error --fail-with-body \
    --user "${PAYPAL_CLIENT_ID}:${PAYPAL_CLIENT_SECRET}" \
    -H 'Accept: application/json' \
    -H 'Accept-Language: en_US' \
    -d 'grant_type=client_credentials' \
    "${PAYPAL_API_BASE}/v1/oauth2/token"); then
    paypal_error "$response"
    return 1
  fi

  token=$(jq -er '.access_token' <<<"$response") || die "OAuth response did not contain an access token"
  printf '%s' "$token"
}

create_order() {
  local access_token=$1
  local amount=$2
  local currency=$3
  local reference=$4
  local body response order_id approval_url

  body=$(jq -n \
    --arg amount "$amount" \
    --arg currency "$currency" \
    --arg reference "$reference" \
    --arg return_url "${PAYPAL_RETURN_URL:-https://example.com/paypal/return}" \
    --arg cancel_url "${PAYPAL_CANCEL_URL:-https://example.com/paypal/cancel}" \
    '{
      intent: "CAPTURE",
      purchase_units: [{
        reference_id: $reference,
        custom_id: $reference,
        description: "Fund Controlled sandbox wallet",
        amount: {
          currency_code: $currency,
          value: $amount
        }
      }],
      payment_source: {
        paypal: {
          experience_context: {
            user_action: "PAY_NOW",
            return_url: $return_url,
            cancel_url: $cancel_url
          }
        }
      }
    }')

  if ! response=$(curl --silent --show-error --fail-with-body \
    -X POST "${PAYPAL_API_BASE}/v2/checkout/orders" \
    -H "Authorization: Bearer ${access_token}" \
    -H 'Content-Type: application/json' \
    -H 'Prefer: return=representation' \
    -H "PayPal-Request-Id: $(request_id)" \
    --data "$body"); then
    paypal_error "$response"
    return 1
  fi

  order_id=$(jq -er '.id' <<<"$response") || die "create-order response did not contain an order ID"
  approval_url=$(jq -er '[.links[] | select(.rel == "payer-action" or .rel == "approve")][0].href' <<<"$response") \
    || die "create-order response did not contain a buyer approval URL"

  printf 'Order created successfully.\n'
  printf 'Order ID: %s\n' "$order_id"
  printf 'Approval URL: %s\n' "$approval_url"

  CREATED_ORDER_ID=$order_id
}

capture_order() {
  local access_token=$1
  local order_id=$2
  local response capture_id status gross currency fee net

  if ! response=$(curl --silent --show-error --fail-with-body \
    -X POST "${PAYPAL_API_BASE}/v2/checkout/orders/${order_id}/capture" \
    -H "Authorization: Bearer ${access_token}" \
    -H 'Content-Type: application/json' \
    -H 'Prefer: return=representation' \
    -H "PayPal-Request-Id: $(request_id)" \
    --data '{}'); then
    paypal_error "$response"
    return 1
  fi

  capture_id=$(jq -er '.purchase_units[0].payments.captures[0].id' <<<"$response") \
    || die "capture response did not contain a capture ID"
  status=$(jq -r '.purchase_units[0].payments.captures[0].status // .status // "UNKNOWN"' <<<"$response")
  gross=$(jq -r '.purchase_units[0].payments.captures[0].seller_receivable_breakdown.gross_amount.value // "unknown"' <<<"$response")
  currency=$(jq -r '.purchase_units[0].payments.captures[0].amount.currency_code // "unknown"' <<<"$response")
  fee=$(jq -r '.purchase_units[0].payments.captures[0].seller_receivable_breakdown.paypal_fee.value // "unknown"' <<<"$response")
  net=$(jq -r '.purchase_units[0].payments.captures[0].seller_receivable_breakdown.net_amount.value // "unknown"' <<<"$response")

  printf 'Order captured.\n'
  printf 'Order ID: %s\n' "$order_id"
  printf 'Capture ID: %s\n' "$capture_id"
  printf 'Status: %s\n' "$status"
  printf 'Gross: %s %s\n' "$gross" "$currency"
  printf 'PayPal fee: %s %s\n' "$fee" "$currency"
  printf 'Net received: %s %s\n' "$net" "$currency"

  [[ "$status" == "COMPLETED" ]] || die "capture was accepted but is not complete (status: ${status})"
}

main() {
  require_command curl
  require_command jq

  local command=${1:-run}
  local access_token amount currency reference order_id

  if [[ "$command" == "-h" || "$command" == "--help" || "$command" == "help" ]]; then
    usage
    return 0
  fi

  [[ -n "${PAYPAL_CLIENT_ID:-}" ]] || die "PAYPAL_CLIENT_ID is required"
  [[ -n "${PAYPAL_CLIENT_SECRET:-}" ]] || die "PAYPAL_CLIENT_SECRET is required"

  case "$command" in
    run|create)
      amount=${2:-10.00}
      currency=${3:-USD}
      [[ "$amount" =~ ^[0-9]+([.][0-9]{1,2})?$ ]] || die "amount must be a positive decimal with at most two fractional digits"
      [[ "$amount" != "0" && "$amount" != "0.0" && "$amount" != "0.00" ]] || die "amount must be greater than zero"
      [[ "$currency" =~ ^[A-Z]{3}$ ]] || die "currency must be a three-letter uppercase code such as USD"
      reference=${CONTROLLED_FUNDING_REFERENCE:-controlled-$(date +%s)-$$}
      access_token=$(get_access_token)
      create_order "$access_token" "$amount" "$currency" "$reference"

      if [[ "$command" == "create" ]]; then
        printf '\nAfter approval, capture with:\n'
        printf '  %q capture %q\n' "$0" "$CREATED_ORDER_ID"
        return 0
      fi

      if [[ ! -t 0 ]]; then
        printf '\nNo interactive terminal detected. After approval, run:\n'
        printf '  %q capture %q\n' "$0" "$CREATED_ORDER_ID"
        return 0
      fi

      printf '\nOpen the approval URL and sign in with the Sandbox Personal account.\n'
      read -r -p 'Press Enter after the buyer approves the payment... '
      capture_order "$access_token" "$CREATED_ORDER_ID"
      ;;
    capture)
      order_id=${2:-}
      [[ -n "$order_id" ]] || die "capture requires an ORDER_ID"
      access_token=$(get_access_token)
      capture_order "$access_token" "$order_id"
      ;;
    *)
      usage >&2
      die "unknown command: $command"
      ;;
  esac
}

main "$@"
