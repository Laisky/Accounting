-- +goose Up
CREATE TABLE users (
    id text PRIMARY KEY,
    email text NOT NULL,
    status text NOT NULL,
    email_verified boolean NOT NULL DEFAULT false,
    totp_enabled boolean NOT NULL DEFAULT false,
    base_currency text NOT NULL DEFAULT '',
    password_hash text NOT NULL,
    totp_secret text NOT NULL DEFAULT '',
    external_sso_subject text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_status_chk CHECK (status IN ('pending_verification','active'))
);
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));
CREATE UNIQUE INDEX users_external_sso_subject_key ON users (external_sso_subject) WHERE external_sso_subject <> '';

CREATE TABLE sessions (
    token_hash text PRIMARY KEY,
    id text NOT NULL,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_email text NOT NULL,
    status text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE books (
    id text PRIMARY KEY,
    owner_user_id text NOT NULL REFERENCES users(id),
    name text NOT NULL,
    reporting_currency text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX books_owner_idx ON books (owner_user_id);

CREATE TABLE book_members (
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role text NOT NULL,
    display_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (book_id, user_id),
    CONSTRAINT book_members_role_chk CHECK (role IN ('owner','administrator','member','viewer'))
);
CREATE INDEX book_members_user_idx ON book_members (user_id);

CREATE TABLE account_groups (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX account_groups_user_idx ON account_groups (user_id);

CREATE TABLE accounts (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id text NOT NULL REFERENCES account_groups(id),
    name text NOT NULL,
    type text NOT NULL,
    currency text NOT NULL,
    opening_balance_cents bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT accounts_type_chk CHECK (type IN ('cash','savings','credit_card','loan','investment','payment_platform'))
);
CREATE INDEX accounts_user_idx ON accounts (user_id);
CREATE INDEX accounts_group_idx ON accounts (group_id);

CREATE TABLE account_shared_books (
    account_id text NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    PRIMARY KEY (account_id, book_id)
);
CREATE INDEX account_shared_books_book_idx ON account_shared_books (book_id);

CREATE TABLE categories (
    id text PRIMARY KEY,
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    parent_id text REFERENCES categories(id) ON DELETE SET NULL,
    name text NOT NULL,
    direction text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    archived boolean NOT NULL DEFAULT false,
    raw_source_name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT categories_direction_chk CHECK (direction IN ('income','expense'))
);
CREATE INDEX categories_book_idx ON categories (book_id);
CREATE INDEX categories_parent_idx ON categories (parent_id);

CREATE TABLE entries (
    id text PRIMARY KEY,
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    creator_user_id text NOT NULL REFERENCES users(id),
    type text NOT NULL,
    account_id text REFERENCES accounts(id),
    destination_account_id text REFERENCES accounts(id),
    category_id text REFERENCES categories(id),
    amount_cents bigint NOT NULL,
    transaction_currency text NOT NULL,
    account_currency text NOT NULL,
    book_reporting_currency text NOT NULL,
    exchange_rate text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL,
    note text NOT NULL DEFAULT '',
    merchant text NOT NULL DEFAULT '',
    tags jsonb NOT NULL DEFAULT '[]'::jsonb,
    raw_source text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT entries_amount_positive_chk CHECK (amount_cents > 0),
    CONSTRAINT entries_type_chk CHECK (type IN ('expense','income','transfer','refund','reimbursement','borrow','lend','repayment')),
    CONSTRAINT entries_transfer_dest_chk CHECK (type <> 'transfer' OR destination_account_id IS NOT NULL)
);
CREATE INDEX entries_book_keyset_idx ON entries (book_id, occurred_at DESC, id DESC);
CREATE INDEX entries_account_idx ON entries (account_id);

CREATE TABLE journal_entries (
    id text PRIMARY KEY,
    entry_id text NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT journal_entries_entry_key UNIQUE (entry_id)
);
CREATE INDEX journal_entries_book_idx ON journal_entries (book_id);

CREATE TABLE postings (
    id text PRIMARY KEY,
    journal_id text NOT NULL REFERENCES journal_entries(id) ON DELETE CASCADE,
    entry_id text NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    book_id text NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    account_id text NOT NULL REFERENCES accounts(id),
    direction text NOT NULL,
    amount_cents bigint NOT NULL,
    currency text NOT NULL,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT postings_direction_chk CHECK (direction IN ('debit','credit')),
    CONSTRAINT postings_amount_positive_chk CHECK (amount_cents > 0)
);
CREATE INDEX postings_journal_idx ON postings (journal_id);
CREATE INDEX postings_account_idx ON postings (account_id, occurred_at);
CREATE INDEX postings_book_idx ON postings (book_id);

CREATE TABLE exchange_rates (
    currency text PRIMARY KEY,
    units_per_usd text NOT NULL,
    source text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE import_batches (
    id text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source text NOT NULL,
    filename text NOT NULL DEFAULT '',
    content_type text NOT NULL DEFAULT '',
    source_hash text NOT NULL,
    parser_version text NOT NULL DEFAULT '',
    status text NOT NULL,
    detected_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    detected jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_count integer NOT NULL DEFAULT 0,
    warning_count integer NOT NULL DEFAULT 0,
    applied_book_id text REFERENCES books(id),
    applied_entry_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    applied_skipped_rows jsonb NOT NULL DEFAULT '[]'::jsonb,
    applied_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT import_batches_status_chk CHECK (status IN ('preview','applied','applying')),
    CONSTRAINT import_batches_hash_key UNIQUE (user_id, source_hash)
);
CREATE INDEX import_batches_user_idx ON import_batches (user_id);

CREATE TABLE import_rows (
    batch_id text NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
    row_number integer NOT NULL,
    data jsonb NOT NULL,
    error_count integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (batch_id, row_number)
);

CREATE TABLE audit_events (
    id text PRIMARY KEY,
    seq bigint GENERATED ALWAYS AS IDENTITY,
    actor_id text,
    actor_email text NOT NULL DEFAULT '',
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_actor_idx ON audit_events (actor_id, created_at DESC);
CREATE UNIQUE INDEX audit_events_seq_key ON audit_events (seq);

-- +goose Down
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS import_rows;
DROP TABLE IF EXISTS import_batches;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS postings;
DROP TABLE IF EXISTS journal_entries;
DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS account_shared_books;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS account_groups;
DROP TABLE IF EXISTS book_members;
DROP TABLE IF EXISTS books;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
