-- +goose Up

-- ---------------------------------------------------------------------------
-- Helpers
-- ---------------------------------------------------------------------------

-- Returns the tenant id for the current transaction, or NULL when unset.
-- Set per transaction with: SET LOCAL app.tenant_id = '<uuid>'
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION app_tenant_id() RETURNS uuid
LANGUAGE sql STABLE
AS $$
  SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- Enums
-- ---------------------------------------------------------------------------
CREATE TYPE user_role          AS ENUM ('owner', 'staff');
CREATE TYPE member_status      AS ENUM ('active', 'inactive', 'frozen');
CREATE TYPE subscription_status AS ENUM ('active', 'expired', 'cancelled');
CREATE TYPE payment_method     AS ENUM ('cash', 'card', 'upi', 'bank_transfer', 'other');
CREATE TYPE pt_package_status  AS ENUM ('active', 'completed', 'expired', 'cancelled');
CREATE TYPE pt_session_status  AS ENUM ('scheduled', 'completed', 'cancelled', 'no_show');

-- ---------------------------------------------------------------------------
-- Platform tables (no RLS; queried before a tenant is known)
-- ---------------------------------------------------------------------------
CREATE TABLE tenants (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name        text NOT NULL,
  slug        text NOT NULL UNIQUE,
  timezone    text NOT NULL DEFAULT 'UTC',
  currency    text NOT NULL DEFAULT 'USD',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER tenants_updated_at BEFORE UPDATE ON tenants
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE users (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  email          text NOT NULL,
  password_hash  text NOT NULL,
  full_name      text NOT NULL,
  role           user_role NOT NULL DEFAULT 'owner',
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_unique ON users (lower(email));
CREATE INDEX users_tenant_idx ON users (tenant_id);
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Tenant-scoped domain tables
-- Every table has tenant_id and UNIQUE (tenant_id, id) so children can use
-- composite FKs that make cross-tenant references impossible.
-- ---------------------------------------------------------------------------
CREATE TABLE trainers (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  full_name    text NOT NULL,
  email        text,
  phone        text,
  specialties  text[] NOT NULL DEFAULT '{}',
  active       boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id)
);
CREATE INDEX trainers_tenant_idx ON trainers (tenant_id, active);
CREATE TRIGGER trainers_updated_at BEFORE UPDATE ON trainers
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE membership_plans (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name           text NOT NULL,
  duration_days  integer NOT NULL CHECK (duration_days > 0),
  price          numeric(12,2) NOT NULL CHECK (price >= 0),
  description    text NOT NULL DEFAULT '',
  active         boolean NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id)
);
CREATE INDEX membership_plans_tenant_idx ON membership_plans (tenant_id, active);
CREATE TRIGGER membership_plans_updated_at BEFORE UPDATE ON membership_plans
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE members (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  full_name          text NOT NULL,
  email              text,
  phone              text,
  date_of_birth      date,
  gender             text,
  joined_at          date NOT NULL DEFAULT CURRENT_DATE,
  status             member_status NOT NULL DEFAULT 'active',
  emergency_contact  text,
  notes              text NOT NULL DEFAULT '',
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id)
);
CREATE INDEX members_tenant_status_idx ON members (tenant_id, status);
CREATE INDEX members_tenant_name_idx ON members (tenant_id, lower(full_name));
CREATE TRIGGER members_updated_at BEFORE UPDATE ON members
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE subscriptions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  member_id   uuid NOT NULL,
  plan_id     uuid NOT NULL,
  start_date  date NOT NULL,
  end_date    date NOT NULL,
  price       numeric(12,2) NOT NULL CHECK (price >= 0),
  status      subscription_status NOT NULL DEFAULT 'active',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  CHECK (end_date >= start_date),
  FOREIGN KEY (tenant_id, member_id) REFERENCES members (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, plan_id)   REFERENCES membership_plans (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX subscriptions_member_idx ON subscriptions (tenant_id, member_id, end_date DESC);
CREATE INDEX subscriptions_end_date_idx ON subscriptions (tenant_id, status, end_date);
CREATE TRIGGER subscriptions_updated_at BEFORE UPDATE ON subscriptions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE pt_packages (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  member_id       uuid NOT NULL,
  trainer_id      uuid NOT NULL,
  name            text NOT NULL,
  total_sessions  integer NOT NULL CHECK (total_sessions > 0),
  price           numeric(12,2) NOT NULL CHECK (price >= 0),
  purchased_at    date NOT NULL DEFAULT CURRENT_DATE,
  expires_at      date,
  status          pt_package_status NOT NULL DEFAULT 'active',
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, member_id)  REFERENCES members (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, trainer_id) REFERENCES trainers (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX pt_packages_member_idx  ON pt_packages (tenant_id, member_id);
CREATE INDEX pt_packages_trainer_idx ON pt_packages (tenant_id, trainer_id, status);
CREATE TRIGGER pt_packages_updated_at BEFORE UPDATE ON pt_packages
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE pt_sessions (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id         uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  package_id        uuid NOT NULL,
  member_id         uuid NOT NULL,
  trainer_id        uuid NOT NULL,
  scheduled_at      timestamptz NOT NULL,
  duration_minutes  integer NOT NULL DEFAULT 60 CHECK (duration_minutes > 0),
  status            pt_session_status NOT NULL DEFAULT 'scheduled',
  notes             text NOT NULL DEFAULT '',
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, package_id) REFERENCES pt_packages (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, member_id)  REFERENCES members (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, trainer_id) REFERENCES trainers (tenant_id, id) ON DELETE RESTRICT
);
CREATE INDEX pt_sessions_schedule_idx ON pt_sessions (tenant_id, scheduled_at);
CREATE INDEX pt_sessions_trainer_idx  ON pt_sessions (tenant_id, trainer_id, scheduled_at);
CREATE INDEX pt_sessions_package_idx  ON pt_sessions (tenant_id, package_id, status);
CREATE TRIGGER pt_sessions_updated_at BEFORE UPDATE ON pt_sessions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE payments (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  member_id        uuid NOT NULL,
  subscription_id  uuid,
  pt_package_id    uuid,
  amount           numeric(12,2) NOT NULL CHECK (amount > 0),
  method           payment_method NOT NULL DEFAULT 'cash',
  paid_at          timestamptz NOT NULL DEFAULT now(),
  notes            text NOT NULL DEFAULT '',
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id),
  FOREIGN KEY (tenant_id, member_id)       REFERENCES members (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, subscription_id) REFERENCES subscriptions (tenant_id, id) ON DELETE SET NULL (subscription_id),
  FOREIGN KEY (tenant_id, pt_package_id)   REFERENCES pt_packages (tenant_id, id) ON DELETE SET NULL (pt_package_id)
);
CREATE INDEX payments_tenant_paid_idx  ON payments (tenant_id, paid_at DESC);
CREATE INDEX payments_member_idx       ON payments (tenant_id, member_id, paid_at DESC);
CREATE INDEX payments_subscription_idx ON payments (tenant_id, subscription_id);
CREATE INDEX payments_package_idx      ON payments (tenant_id, pt_package_id);

-- ---------------------------------------------------------------------------
-- Row-level security: enabled AND forced so the table owner is restricted too.
-- ---------------------------------------------------------------------------
ALTER TABLE trainers         ENABLE ROW LEVEL SECURITY;
ALTER TABLE trainers         FORCE  ROW LEVEL SECURITY;
ALTER TABLE membership_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE membership_plans FORCE  ROW LEVEL SECURITY;
ALTER TABLE members          ENABLE ROW LEVEL SECURITY;
ALTER TABLE members          FORCE  ROW LEVEL SECURITY;
ALTER TABLE subscriptions    ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions    FORCE  ROW LEVEL SECURITY;
ALTER TABLE pt_packages      ENABLE ROW LEVEL SECURITY;
ALTER TABLE pt_packages      FORCE  ROW LEVEL SECURITY;
ALTER TABLE pt_sessions      ENABLE ROW LEVEL SECURITY;
ALTER TABLE pt_sessions      FORCE  ROW LEVEL SECURITY;
ALTER TABLE payments         ENABLE ROW LEVEL SECURITY;
ALTER TABLE payments         FORCE  ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON trainers
  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
CREATE POLICY tenant_isolation ON membership_plans
  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
CREATE POLICY tenant_isolation ON members
  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
CREATE POLICY tenant_isolation ON subscriptions
  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
CREATE POLICY tenant_isolation ON pt_packages
  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
CREATE POLICY tenant_isolation ON pt_sessions
  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
CREATE POLICY tenant_isolation ON payments
  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- +goose Down
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS pt_sessions;
DROP TABLE IF EXISTS pt_packages;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS membership_plans;
DROP TABLE IF EXISTS trainers;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;
DROP TYPE IF EXISTS pt_session_status;
DROP TYPE IF EXISTS pt_package_status;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS subscription_status;
DROP TYPE IF EXISTS member_status;
DROP TYPE IF EXISTS user_role;
DROP FUNCTION IF EXISTS set_updated_at();
DROP FUNCTION IF EXISTS app_tenant_id();
