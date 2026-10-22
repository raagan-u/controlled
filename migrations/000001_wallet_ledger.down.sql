BEGIN;

DROP VIEW IF EXISTS ledger_account_balances;
DROP TRIGGER IF EXISTS ledger_entries_append_only ON ledger_entries;
DROP TRIGGER IF EXISTS ledger_transactions_append_only ON ledger_transactions;
DROP FUNCTION IF EXISTS reject_ledger_mutation();
DROP TRIGGER IF EXISTS ledger_account_nonnegative_on_entry ON ledger_entries;
DROP FUNCTION IF EXISTS assert_nonnegative_ledger_account_balance();
DROP TRIGGER IF EXISTS ledger_transaction_balance_on_entry ON ledger_entries;
DROP TRIGGER IF EXISTS ledger_transaction_balance_on_transaction ON ledger_transactions;
DROP FUNCTION IF EXISTS assert_balanced_ledger_transaction();
DROP TRIGGER IF EXISTS ledger_entry_currency_check ON ledger_entries;
DROP FUNCTION IF EXISTS enforce_ledger_entry_currency();
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS ledger_transactions;
DROP TABLE IF EXISTS ledger_accounts;
DROP TABLE IF EXISTS wallets;
DROP TABLE IF EXISTS users;

COMMIT;
