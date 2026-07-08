-- +goose Up
-- Add the reporting-currency amount to postings so per-journal debit=credit is
-- verifiable in pure SQL. amount_cents stays in the leg's own (account/transaction)
-- currency; reporting_cents holds that leg converted to the book reporting currency.
ALTER TABLE postings ADD COLUMN reporting_cents bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE postings DROP COLUMN reporting_cents;
