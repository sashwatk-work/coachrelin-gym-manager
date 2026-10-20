-- +goose Up

-- ---------------------------------------------------------------------------
-- PT plans: reusable personal-training package templates, the PT counterpart
-- of membership_plans. Buying a package for a member starts from a plan
-- (sessions, validity, price) which can then be customised; the package row
-- keeps its own snapshot of those values so later edits to the plan do not
-- change packages already sold.
-- ---------------------------------------------------------------------------

CREATE TABLE pt_plans (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name           text NOT NULL,
  sessions       integer NOT NULL CHECK (sessions > 0),
  validity_days  integer CHECK (validity_days IS NULL OR validity_days > 0), -- NULL = never expires
  price          numeric(12,2) NOT NULL CHECK (price >= 0),
  description    text NOT NULL DEFAULT '',
  active         boolean NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, id)
);
CREATE INDEX pt_plans_tenant_idx ON pt_plans (tenant_id, active);
CREATE TRIGGER pt_plans_updated_at BEFORE UPDATE ON pt_plans
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE pt_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE pt_plans FORCE  ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON pt_plans
  USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- Which plan a package was bought from (NULL = custom package).
ALTER TABLE pt_packages ADD COLUMN pt_plan_id uuid;
ALTER TABLE pt_packages
  ADD CONSTRAINT pt_packages_plan_fkey
  FOREIGN KEY (tenant_id, pt_plan_id) REFERENCES pt_plans (tenant_id, id) ON DELETE RESTRICT;
CREATE INDEX pt_packages_plan_idx ON pt_packages (tenant_id, pt_plan_id);

-- Give every existing gym the same starter catalogue the seeder uses, so a
-- gym that was created before PT plans existed is not left with an empty list.
-- RLS is forced on this table, so it has to be off for a cross-tenant insert.
ALTER TABLE pt_plans NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pt_plans DISABLE ROW LEVEL SECURITY;
INSERT INTO pt_plans (tenant_id, name, sessions, validity_days, price, description, active)
SELECT t.id, x.name, x.sessions, x.validity_days, x.price, x.description, true
FROM tenants t
CROSS JOIN (VALUES
  ('4-session PT pack',  4,  60,  3600.00, 'Four 1-hour sessions. A short block to try a trainer.'),
  ('8-session PT pack',  8,  90,  6400.00, 'Eight 1-hour sessions. Valid for 3 months.'),
  ('12-session PT pack', 12, 90,  9000.00, 'Twelve 1-hour sessions. The most popular pack.'),
  ('20-session PT pack', 20, 120, 14000.00, 'Twenty 1-hour sessions. Best value, valid for 4 months.')
) AS x(name, sessions, validity_days, price, description)
WHERE NOT EXISTS (SELECT 1 FROM pt_plans p WHERE p.tenant_id = t.id);
ALTER TABLE pt_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE pt_plans FORCE ROW LEVEL SECURITY;

-- +goose Down
ALTER TABLE pt_packages DROP CONSTRAINT IF EXISTS pt_packages_plan_fkey;
DROP INDEX IF EXISTS pt_packages_plan_idx;
ALTER TABLE pt_packages DROP COLUMN IF EXISTS pt_plan_id;
DROP TABLE IF EXISTS pt_plans;
