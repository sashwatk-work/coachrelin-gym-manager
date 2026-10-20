-- +goose Up

ALTER TABLE tenants
  ADD COLUMN gst_percent numeric(5,2) NOT NULL DEFAULT 0
  CHECK (gst_percent >= 0 AND gst_percent <= 100);

-- +goose Down

ALTER TABLE tenants DROP COLUMN gst_percent;
