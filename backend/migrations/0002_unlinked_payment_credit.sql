-- +goose Up

-- ---------------------------------------------------------------------------
-- Unlinked payments as account credit
--
-- A payment may be recorded without a subscription_id / pt_package_id
-- ("General"). Such payments must still reduce what the member owes, so they
-- are treated as credit on the member's account and applied to the member's
-- billable items (non-cancelled subscriptions and PT packages) oldest first,
-- until the credit runs out. These helpers make that allocation available to
-- the queries that compute per-item "paid" and per-member dues.
--
-- All functions are plain SQL (not SECURITY DEFINER), so row-level security
-- continues to apply to the tables they read.
-- ---------------------------------------------------------------------------

-- Total of a member's payments that are not linked to any specific item.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION member_unlinked_credit(mid uuid) RETURNS numeric
LANGUAGE sql STABLE
AS $$
  SELECT COALESCE(SUM(amount), 0)
  FROM payments
  WHERE member_id = mid AND subscription_id IS NULL AND pt_package_id IS NULL
$$;
-- +goose StatementEnd

-- Outstanding balance (price minus directly linked payments) of all of a
-- member's billable items created strictly before the given item, i.e. the
-- amount of unlinked credit that older items consume first.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION member_balance_before(mid uuid, item_created timestamptz, item_id uuid) RETURNS numeric
LANGUAGE sql STABLE
AS $$
  SELECT COALESCE(SUM(GREATEST(x.price - x.paid, 0)), 0)
  FROM (
    SELECT s.price, s.created_at, s.id,
           COALESCE((SELECT SUM(amount) FROM payments p WHERE p.subscription_id = s.id), 0) AS paid
    FROM subscriptions s
    WHERE s.member_id = mid AND s.status <> 'cancelled'
    UNION ALL
    SELECT pk.price, pk.created_at, pk.id,
           COALESCE((SELECT SUM(amount) FROM payments p WHERE p.pt_package_id = pk.id), 0)
    FROM pt_packages pk
    WHERE pk.member_id = mid AND pk.status <> 'cancelled'
  ) x
  WHERE (x.created_at, x.id) < (item_created, item_id)
$$;
-- +goose StatementEnd

-- Unlinked credit allocated to one item: whatever credit is left after older
-- items are covered, capped at this item's own remaining balance.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION allocated_credit(mid uuid, item_created timestamptz, item_id uuid, remaining numeric) RETURNS numeric
LANGUAGE sql STABLE
AS $$
  SELECT LEAST(
    GREATEST(member_unlinked_credit(mid) - member_balance_before(mid, item_created, item_id), 0),
    GREATEST(remaining, 0)
  )
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS allocated_credit(uuid, timestamptz, uuid, numeric);
DROP FUNCTION IF EXISTS member_balance_before(uuid, timestamptz, uuid);
DROP FUNCTION IF EXISTS member_unlinked_credit(uuid);
