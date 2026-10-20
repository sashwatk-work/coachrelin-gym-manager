-- +goose Up

-- Replace the catch-all "staff" role with cashier (front desk) and trainer
-- (login linked to a trainers row). Existing staff users become cashiers.

ALTER TABLE users
  ADD COLUMN trainer_id uuid REFERENCES trainers(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX users_trainer_id_unique ON users (trainer_id) WHERE trainer_id IS NOT NULL;

ALTER TABLE users ALTER COLUMN role DROP DEFAULT;

ALTER TYPE user_role RENAME TO user_role_old;
CREATE TYPE user_role AS ENUM ('owner', 'cashier', 'trainer');

ALTER TABLE users
  ALTER COLUMN role TYPE user_role
  USING (
    CASE role::text
      WHEN 'staff' THEN 'cashier'::user_role
      WHEN 'owner' THEN 'owner'::user_role
      ELSE 'cashier'::user_role
    END
  );

ALTER TABLE users ALTER COLUMN role SET DEFAULT 'owner'::user_role;

DROP TYPE user_role_old;

-- +goose Down

ALTER TABLE users ALTER COLUMN role DROP DEFAULT;

ALTER TYPE user_role RENAME TO user_role_old;
CREATE TYPE user_role AS ENUM ('owner', 'staff');

ALTER TABLE users
  ALTER COLUMN role TYPE user_role
  USING (
    CASE role::text
      WHEN 'cashier' THEN 'staff'::user_role
      WHEN 'trainer' THEN 'staff'::user_role
      ELSE 'owner'::user_role
    END
  );

ALTER TABLE users ALTER COLUMN role SET DEFAULT 'owner'::user_role;
DROP TYPE user_role_old;

DROP INDEX IF EXISTS users_trainer_id_unique;
ALTER TABLE users DROP COLUMN IF EXISTS trainer_id;
