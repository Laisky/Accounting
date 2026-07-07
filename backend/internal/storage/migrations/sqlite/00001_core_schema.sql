-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    status TEXT NOT NULL,
    email_verified INTEGER NOT NULL DEFAULT 0 CHECK (email_verified IN (0,1)),
    totp_enabled INTEGER NOT NULL DEFAULT 0 CHECK (totp_enabled IN (0,1)),
    base_currency TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    totp_secret TEXT NOT NULL DEFAULT '',
    external_sso_subject TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CONSTRAINT users_status_chk CHECK (status IN ('pending_verification','active'))
);
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));
CREATE UNIQUE INDEX users_external_sso_subject_key ON users (external_sso_subject) WHERE external_sso_subject <> '';

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_email TEXT NOT NULL,
    status TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE books (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    reporting_currency TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX books_owner_idx ON books (owner_user_id);

CREATE TABLE book_members (
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (book_id, user_id),
    CONSTRAINT book_members_role_chk CHECK (role IN ('owner','administrator','member','viewer'))
);
CREATE INDEX book_members_user_idx ON book_members (user_id);

CREATE TABLE account_groups (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX account_groups_user_idx ON account_groups (user_id);

CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES account_groups(id),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    currency TEXT NOT NULL,
    opening_balance_cents INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CONSTRAINT accounts_type_chk CHECK (type IN ('cash','savings','credit_card','loan','investment','payment_platform'))
);
CREATE INDEX accounts_user_idx ON accounts (user_id);
CREATE INDEX accounts_group_idx ON accounts (group_id);

CREATE TABLE account_shared_books (
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    PRIMARY KEY (account_id, book_id)
);
CREATE INDEX account_shared_books_book_idx ON account_shared_books (book_id);

CREATE TABLE categories (
    id TEXT PRIMARY KEY,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    parent_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    direction TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    archived INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0,1)),
    raw_source_name TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CONSTRAINT categories_direction_chk CHECK (direction IN ('income','expense'))
);
CREATE INDEX categories_book_idx ON categories (book_id);
CREATE INDEX categories_parent_idx ON categories (parent_id);

CREATE TABLE entries (
    id TEXT PRIMARY KEY,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    creator_user_id TEXT NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    account_id TEXT REFERENCES accounts(id),
    destination_account_id TEXT REFERENCES accounts(id),
    category_id TEXT REFERENCES categories(id),
    amount_cents INTEGER NOT NULL,
    transaction_currency TEXT NOT NULL,
    account_currency TEXT NOT NULL,
    book_reporting_currency TEXT NOT NULL,
    exchange_rate TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    merchant TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(tags)),
    raw_source TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CONSTRAINT entries_amount_positive_chk CHECK (amount_cents > 0),
    CONSTRAINT entries_type_chk CHECK (type IN ('expense','income','transfer','refund','reimbursement','borrow','lend','repayment')),
    CONSTRAINT entries_transfer_dest_chk CHECK (type <> 'transfer' OR destination_account_id IS NOT NULL)
);
CREATE INDEX entries_book_keyset_idx ON entries (book_id, occurred_at DESC, id DESC);
CREATE INDEX entries_account_idx ON entries (account_id);

CREATE TABLE journal_entries (
    id TEXT PRIMARY KEY,
    entry_id TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CONSTRAINT journal_entries_entry_key UNIQUE (entry_id)
);
CREATE INDEX journal_entries_book_idx ON journal_entries (book_id);

CREATE TABLE postings (
    id TEXT PRIMARY KEY,
    journal_id TEXT NOT NULL REFERENCES journal_entries(id) ON DELETE CASCADE,
    entry_id TEXT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    direction TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    currency TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CONSTRAINT postings_direction_chk CHECK (direction IN ('debit','credit')),
    CONSTRAINT postings_amount_positive_chk CHECK (amount_cents > 0)
);
CREATE INDEX postings_journal_idx ON postings (journal_id);
CREATE INDEX postings_account_idx ON postings (account_id, occurred_at);
CREATE INDEX postings_book_idx ON postings (book_id);

CREATE TABLE exchange_rates (
    currency TEXT PRIMARY KEY,
    units_per_usd TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE import_batches (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    source_hash TEXT NOT NULL,
    parser_version TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    detected_schema TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(detected_schema)),
    detected TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(detected)),
    error_count INTEGER NOT NULL DEFAULT 0,
    warning_count INTEGER NOT NULL DEFAULT 0,
    applied_book_id TEXT REFERENCES books(id),
    applied_entry_ids TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(applied_entry_ids)),
    applied_skipped_rows TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(applied_skipped_rows)),
    applied_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    CONSTRAINT import_batches_status_chk CHECK (status IN ('preview','applied','applying')),
    CONSTRAINT import_batches_hash_key UNIQUE (user_id, source_hash)
);
CREATE INDEX import_batches_user_idx ON import_batches (user_id);

CREATE TABLE import_rows (
    batch_id TEXT NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
    row_number INTEGER NOT NULL,
    data TEXT NOT NULL CHECK (json_valid(data)),
    error_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (batch_id, row_number)
);

CREATE TABLE audit_events (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    actor_id TEXT,
    actor_email TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
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
