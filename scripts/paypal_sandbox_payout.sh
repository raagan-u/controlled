#!/usr/bin/env bash

set -Eeuo pipefail

readonly PAYPAL_API_BASE="${PAYPAL_API_BASE:-https://api-m.sandbox.paypal.com}"

usage() {
  cat <<'EOF'
Test a PayPal Sandbox withdrawal (Controlled Business -> Personal PayPal account).

Usage:
  PAYPAL_CLIENT_ID=... PAYPAL_CLIENT_SECRET=... \
    scripts/paypal_sandbox_payout.sh send RECIPIENT_EMAIL [AMOUNT] [CURRENCY]

  PAYPAL_CLIENT_ID=... PAYPAL_CLIENT_SECRET=... \
    scripts/paypal_sandbox_payout.sh status PAYOUT_BATCH_ID

Commands:
  send    Send one payout and show its initial batch/item status.
  status  Retrieve the latest batch and item status.

Defaults:
  AMOUNT=1.00
  CURRENCY=USD

Use the email address of the recipient's Sandbox Personal account. Payouts are
separate transfers; they do not refund an Orders v2 capture or its funding source.
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

show_status() {
  local response=$1
  local batch_id batch_status item_id transaction_status transaction_id amount currency fee receiver error_name error_message

  batch_id=$(jq -r '.batch_header.payout_batch_id // "unknown"' <<<"$response")
  batch_status=$(jq -r '.batch_header.batch_status // "unknown"' <<<"$response")
  item_id=$(jq -r '.items[0].payout_item_id // "not-yet-assigned"' <<<"$response")
  transaction_status=$(jq -r '.items[0].transaction_status // "not-yet-available"' <<<"$response")
  transaction_id=$(jq -r '.items[0].transaction_id // "not-yet-assigned"' <<<"$response")
  amount=$(jq -r '.items[0].payout_item.amount.value // .batch_header.amount.value // "unknown"' <<<"$response")
  currency=$(jq -r '.items[0].payout_item.amount.currency // .batch_header.amount.currency // "unknown"' <<<"$response")
  fee=$(jq -r '.items[0].payout_item_fee.value // .batch_header.fees.value // "not-yet-available"' <<<"$response")
  receiver=$(jq -r '.items[0].payout_item.receiver // "not-returned"' <<<"$response")
  error_name=$(jq -r '.items[0].errors.name // empty' <<<"$response")
  error_message=$(jq -r '.items[0].errors.message // empty' <<<"$response")

  printf 'Payout batch ID: %s\n' "$batch_id"
  printf 'Batch status: %s\n' "$batch_status"
  printf 'Payout item ID: %s\n' "$item_id"
  printf 'Item status: %s\n' "$transaction_status"
  printf 'Transaction ID: %s\n' "$transaction_id"
  printf 'Recipient: %s\n' "$receiver"
  printf 'Amount: %s %s\n' "$amount" "$currency"
  printf 'PayPal fee: %s %s\n' "$fee" "$currency"

  if [[ -n "$error_name" ]]; then
    printf 'Item error: %s - %s\n' "$error_name" "$error_message" >&2
  fi
}

get_batch() {
  local access_token=$1
  local batch_id=$2
  local response

  if ! response=$(curl --silent --show-error --fail-with-body \
    "${PAYPAL_API_BASE}/v1/payments/payouts/${batch_id}?page_size=100&page=1&total_required=true" \
    -H "Authorization: Bearer ${access_token}" \
    -H 'Content-Type: application/json'); then
    paypal_error "$response"
    return 1
  fi

  show_status "$response"
}

send_payout() {
  local access_token=$1
  local receiver=$2
  local amount=$3
  local currency=$4
  local batch_reference=$5
  local item_reference=$6
  local body response batch_id

  body=$(jq -n \
    --arg batch_reference "$batch_reference" \
    --arg item_reference "$item_reference" \
    --arg receiver "$receiver" \
    --arg amount "$amount" \
    --arg currency "$currency" \
    '{
      sender_batch_header: {
        sender_batch_id: $batch_reference,
        email_subject: "Your Controlled Sandbox withdrawal",
        email_message: "A Controlled Sandbox withdrawal was sent to your PayPal account."
      },
      items: [{
        recipient_type: "EMAIL",
        receiver: $receiver,
        sender_item_id: $item_reference,
        amount: {
          value: $amount,
          currency: $currency
        },
        note: "Controlled Sandbox wallet withdrawal"
      }]
    }')

  if ! response=$(curl --silent --show-error --fail-with-body \
    -X POST "${PAYPAL_API_BASE}/v1/payments/payouts" \
    -H "Authorization: Bearer ${access_token}" \
    -H 'Content-Type: application/json' \
    -H "PayPal-Request-Id: $(request_id)" \
    --data "$body"); then
    paypal_error "$response"
    return 1
  fi

  batch_id=$(jq -er '.batch_header.payout_batch_id' <<<"$response") \
    || die "create-payout response did not contain a payout batch ID"

  printf 'Payout accepted by PayPal.\n'
  printf 'Payout batch ID: %s\n' "$batch_id"
  printf 'Initial batch status: %s\n' "$(jq -r '.batch_header.batch_status // "UNKNOWN"' <<<"$response")"
  printf '\nRetrieving item status...\n'
  get_batch "$access_token" "$batch_id"
  printf '\nIf the item is still pending, check it again with:\n'
  printf '  %q status %q\n' "$0" "$batch_id"
}

main() {
  require_command curl
  require_command jq

  local command=${1:-help}
  local access_token receiver amount currency batch_id unique_suffix

  if [[ "$command" == "-h" || "$command" == "--help" || "$command" == "help" ]]; then
    usage
    return 0
  fi

  [[ -n "${PAYPAL_CLIENT_ID:-}" ]] || die "PAYPAL_CLIENT_ID is required"
  [[ -n "${PAYPAL_CLIENT_SECRET:-}" ]] || die "PAYPAL_CLIENT_SECRET is required"

  case "$command" in
    send)
      receiver=${2:-}
      amount=${3:-1.00}
      currency=${4:-USD}
      [[ "$receiver" =~ ^[^[:space:]@]+@[^[:space:]@]+[.][^[:space:]@]+$ ]] || die "send requires a valid Sandbox recipient email"
      [[ "$amount" =~ ^[0-9]+([.][0-9]{1,2})?$ ]] || die "amount must be a positive decimal with at most two fractional digits"
      [[ "$amount" != "0" && "$amount" != "0.0" && "$amount" != "0.00" ]] || die "amount must be greater than zero"
      [[ "$currency" =~ ^[A-Z]{3}$ ]] || die "currency must be a three-letter uppercase code such as USD"
      unique_suffix="$(date +%s)-$$"
      access_token=$(get_access_token)
      send_payout \
        "$access_token" \
        "$receiver" \
        "$amount" \
        "$currency" \
        "controlled-withdrawal-${unique_suffix}" \
        "controlled-item-${unique_suffix}"
      ;;
    status)
      batch_id=${2:-}
      [[ -n "$batch_id" ]] || die "status requires a PAYOUT_BATCH_ID"
      access_token=$(get_access_token)
      get_batch "$access_token" "$batch_id"
      ;;
    *)
      usage >&2
      die "unknown command: $command"
      ;;
  esac
}

main "$@"
