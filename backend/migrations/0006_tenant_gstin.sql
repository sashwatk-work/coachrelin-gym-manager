-- +goose Up

ALTER TABLE tenants ADD COLUMN gstin text;

-- +goose Down

ALTER TABLE tenants DROP COLUMN gstin;
