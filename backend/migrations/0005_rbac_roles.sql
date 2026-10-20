-- +goose Up

-- A login can hold several roles. Cashier is renamed receptionist.

ALTER TYPE user_role RENAME VALUE 'cashier' TO 'receptionist';

ALTER TABLE users ADD COLUMN roles user_role[];

UPDATE users SET roles = ARRAY[role];

ALTER TABLE users
  ALTER COLUMN roles SET NOT NULL,
  ALTER COLUMN roles SET DEFAULT ARRAY['receptionist']::user_role[],
  ADD CONSTRAINT users_roles_not_empty CHECK (cardinality(roles) >= 1);

ALTER TABLE users DROP COLUMN role;

-- +goose Down

ALTER TABLE users ADD COLUMN role user_role NOT NULL DEFAULT 'owner';

UPDATE users SET role = CASE
  WHEN 'owner' = ANY(roles) THEN 'owner'::user_role
  WHEN 'receptionist' = ANY(roles) THEN 'receptionist'::user_role
  ELSE 'trainer'::user_role
END;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_roles_not_empty;
ALTER TABLE users DROP COLUMN roles;

ALTER TYPE user_role RENAME VALUE 'receptionist' TO 'cashier';
