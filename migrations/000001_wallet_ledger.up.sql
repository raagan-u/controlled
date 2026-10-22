CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL,
    display_name text NOT NULL CHECK (btrim(display_name) <> ''),
    created_at timestamptz NOT NULL,
    CHECK (email = lower(btrim(email))),
    CHECK (position('@' IN email) > 1)
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email));

CREATE TABLE wallets (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL UNIQUE REFERENCES users (id),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at timestamptz NOT NULL
);

CREATE TABLE ledger_accounts (
    id uuid PRIMARY KEY,
    wallet_id uuid REFERENCES wallets (id),
    kind text NOT NULL CHECK (kind IN (
        'CUSTODY_ASSET',
        'AVAILABLE',
        'PENDING',
        'COMMITMENT_RESERVE',
        'STAKE_VAULT'
    )),
    normal_side text NOT NULL CHECK (normal_side IN ('DEBIT', 'CREDIT')),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_at timestamptz NOT NULL,
    CHECK (
        (kind IN ('AVAILABLE', 'PENDING', 'COMMITMENT_RESERVE') AND wallet_id IS NOT NULL)
        OR
        (kind IN ('CUSTODY_ASSET', 'STAKE_VAULT') AND wallet_id IS NULL)
    ),
    CHECK (
        (kind = 'CUSTODY_ASSET' AND normal_side = 'DEBIT')
        OR
        (kind <> 'CUSTODY_ASSET' AND normal_side = 'CREDIT')
    )
);

CREATE UNIQUE INDEX ledger_wallet_account_kind_unique
    ON ledger_accounts (wallet_id, kind)
    WHERE wallet_id IS NOT NULL;

CREATE UNIQUE INDEX ledger_system_account_kind_currency_unique
    ON ledger_accounts (kind, currency)
    WHERE wallet_id IS NULL;

CREATE TABLE ledger_transactions (
    id uuid PRIMARY KEY,
    idempotency_key text NOT NULL UNIQUE CHECK (btrim(idempotency_key) <> ''),
    reference_type text NOT NULL CHECK (btrim(reference_type) <> ''),
    reference_id text NOT NULL CHECK (btrim(reference_id) <> ''),
    description text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL,
    posted_at timestamptz NOT NULL DEFAULT now(),
    reverses_transaction_id uuid UNIQUE REFERENCES ledger_transactions (id),
    CHECK (reverses_transaction_id IS NULL OR reverses_transaction_id <> id),
    CHECK ((reference_type = 'REVERSAL') = (reverses_transaction_id IS NOT NULL))
);

CREATE UNIQUE INDEX ledger_reference_unique
    ON ledger_transactions (reference_type, reference_id)
    WHERE reference_type <> 'REVERSAL';

CREATE TABLE ledger_entries (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transaction_id uuid NOT NULL REFERENCES ledger_transactions (id) ON DELETE RESTRICT,
    line_number smallint NOT NULL CHECK (line_number > 0),
    account_id uuid NOT NULL REFERENCES ledger_accounts (id) ON DELETE RESTRICT,
    side text NOT NULL CHECK (side IN ('DEBIT', 'CREDIT')),
    amount_minor bigint NOT NULL CHECK (amount_minor > 0),
    currency text NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    UNIQUE (transaction_id, line_number)
);

CREATE INDEX ledger_entries_account_id_index ON ledger_entries (account_id, id);

CREATE TABLE idempotency_keys (
    scope text NOT NULL CHECK (btrim(scope) <> ''),
    key text NOT NULL CHECK (btrim(key) <> ''),
    request_hash text NOT NULL CHECK (btrim(request_hash) <> ''),
    response_status integer,
    response_body jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    PRIMARY KEY (scope, key),
    CHECK (expires_at IS NULL OR expires_at > created_at),
    CHECK (
        (response_status IS NULL AND response_body IS NULL)
        OR
        (response_status BETWEEN 100 AND 599)
    )
);

CREATE FUNCTION enforce_ledger_entry_currency()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    account_currency text;
BEGIN
    SELECT currency
      INTO account_currency
      FROM ledger_accounts
     WHERE id = NEW.account_id;

    IF account_currency IS NULL OR account_currency <> NEW.currency THEN
        RAISE EXCEPTION 'ledger entry currency must match account currency'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER ledger_entry_currency_check
BEFORE INSERT OR UPDATE ON ledger_entries
FOR EACH ROW
EXECUTE FUNCTION enforce_ledger_entry_currency();

CREATE FUNCTION assert_balanced_ledger_transaction()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    journal_id uuid;
    entry_count bigint;
    currency_count bigint;
    debit_total numeric;
    credit_total numeric;
BEGIN
    IF TG_TABLE_NAME = 'ledger_entries' THEN
        journal_id := NEW.transaction_id;
    ELSE
        journal_id := NEW.id;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM ledger_transactions WHERE id = journal_id) THEN
        RETURN NULL;
    END IF;

    SELECT count(*),
           count(DISTINCT currency),
           COALESCE(sum(amount_minor) FILTER (WHERE side = 'DEBIT'), 0),
           COALESCE(sum(amount_minor) FILTER (WHERE side = 'CREDIT'), 0)
      INTO entry_count, currency_count, debit_total, credit_total
      FROM ledger_entries
     WHERE transaction_id = journal_id;

    IF entry_count < 2 OR currency_count <> 1 OR debit_total <> credit_total THEN
        RAISE EXCEPTION 'ledger transaction % must contain one currency and balanced debits and credits', journal_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER ledger_transaction_balance_on_transaction
AFTER INSERT ON ledger_transactions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION assert_balanced_ledger_transaction();

CREATE CONSTRAINT TRIGGER ledger_transaction_balance_on_entry
AFTER INSERT ON ledger_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION assert_balanced_ledger_transaction();

CREATE FUNCTION assert_nonnegative_ledger_account_balance()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    account_balance numeric;
BEGIN
    SELECT COALESCE(
               sum(
                   CASE
                       WHEN entry.side = account.normal_side THEN entry.amount_minor
                       ELSE -entry.amount_minor
                   END
               ),
               0
           )
      INTO account_balance
      FROM ledger_accounts AS account
      LEFT JOIN ledger_entries AS entry ON entry.account_id = account.id
     WHERE account.id = NEW.account_id
     GROUP BY account.id;

    IF account_balance < 0 THEN
        RAISE EXCEPTION 'ledger posting would make account % negative', NEW.account_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;

-- Posting code must lock every affected ledger_accounts row in a stable order.
-- This deferred check protects the final state; row locks prevent concurrent
-- postings from independently spending the same available balance.
CREATE CONSTRAINT TRIGGER ledger_account_nonnegative_on_entry
AFTER INSERT ON ledger_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION assert_nonnegative_ledger_account_balance();

CREATE FUNCTION reject_ledger_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'posted ledger records are append-only; post a reversal instead'
        USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER ledger_transactions_append_only
BEFORE UPDATE OR DELETE ON ledger_transactions
FOR EACH ROW
EXECUTE FUNCTION reject_ledger_mutation();

CREATE TRIGGER ledger_entries_append_only
BEFORE UPDATE OR DELETE ON ledger_entries
FOR EACH ROW
EXECUTE FUNCTION reject_ledger_mutation();

CREATE VIEW ledger_account_balances AS
SELECT account.id AS account_id,
       account.wallet_id,
       account.kind,
       account.currency,
       COALESCE(
           sum(
               CASE
                   WHEN entry.side = account.normal_side THEN entry.amount_minor
                   ELSE -entry.amount_minor
               END
           ),
           0
       )::bigint AS amount_minor
  FROM ledger_accounts AS account
  LEFT JOIN ledger_entries AS entry ON entry.account_id = account.id
 GROUP BY account.id;
