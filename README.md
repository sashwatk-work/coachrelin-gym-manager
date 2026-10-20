# Coach Relin Fitness

Operations console for **Coach Relin Fitness**. Tracks members, membership plans
and subscriptions, payments, trainers, and personal-training packages and sessions.

This is Relin’s product, not a public gym-manager SaaS. Relin is the owner. Each
location is a tenant with its own members and staff; more gyms can be added later
without mixing data.

- **Backend:** Go 1.25, chi, pgx, PostgreSQL 16 with row-level security per gym
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS, TanStack Query, React Router

## Quick start (local dev)

Prerequisites: Go 1.25+, Node 20+, Docker.

```bash
make db-up        # start Postgres in Docker (127.0.0.1:5432)
make migrate      # apply schema and create Coach Relin Fitness + Relin’s login
make api          # http://localhost:8080
make web          # http://localhost:5173 (proxies /api to :8080)
```

Owner login:

- Email: `owner@coachrelin.fit`
- Password: `password123`

The gym starts empty (no members). Relin adds receptionist and trainer logins from Settings. A login can hold more than one role.

Optional sample members for local demos:

```bash
make seed
```

That also creates:

- Receptionist: `receptionist@coachrelin.fit`
- Trainer: `trainer@coachrelin.fit` (linked to Arjun Nair)

Password for those accounts is also `password123`.

## Run everything in Docker

Copy `docker/.env.example` to `docker/.env` once, then:

```bash
make up           # postgres + backend + frontend (http://127.0.0.1:5173)
make down
```

Docker starts the same empty Coach Relin Fitness gym. Use `make seed` if you want sample members.

Droplet / TLS stack (Nginx reverse proxy, no published database port):

```bash
cp docker/.env.production.example docker/.env.production
make production-up
make production-down
```

## Tests

```bash
make test-go      # unit tests (auth, validation, error mapping, route guards)
make test-go-db   # + HTTP integration tests and DB-level RLS isolation tests (needs `make db-up`)
make test-web     # typecheck + production build
```

The DB-backed tests create throwaway gyms over the HTTP register endpoint (kept for
tests and future locations, not shown in the UI), exercise the membership → payment
→ PT package → session flow, assert that a second gym can neither see nor touch the
first gym’s rows, and verify at the SQL level that RLS is enabled + forced on every
domain table. They clean up after themselves.

## Multi-gym model

- Shared schema; every domain table carries `tenant_id`.
- Postgres row-level security is **enabled and forced** on every domain table, with
  policies comparing `tenant_id` to `current_setting('app.tenant_id')`.
- Each request runs inside a transaction that first executes `SET LOCAL app.tenant_id`
  using the gym from the authenticated session’s JWT. Tenant is never taken from
  client-supplied input.
- Composite foreign keys (`(tenant_id, member_id) -> members(tenant_id, id)`) make
  cross-gym references impossible at the database level.

## Project layout

```
backend/
  cmd/api            entrypoint (flags: -migrate, -seed)
  internal/api       chi router and HTTP handlers
  internal/auth      password hashing, JWT, session middleware
  internal/config    environment config
  internal/db        pgx pool, migrations, tenant transaction helper
  internal/seed      Relin gym bootstrap and optional demo members
  internal/store     SQL repositories
  migrations/        goose SQL migrations
frontend/
  src/api            API client and TanStack Query hooks
  src/components     UI primitives and app shell
  src/pages          route pages
```

## Environment

Local API (`make api`) and `docker/.env`:

| Variable        | Default                                              |
|-----------------|------------------------------------------------------|
| `DATABASE_URL`  | `postgres://gym:gym@127.0.0.1:5432/gym?sslmode=disable` |
| `JWT_SECRET`    | `dev-secret-change-me` (set a real one in production)|
| `PORT`          | `8080`                                               |
| `COOKIE_SECURE` | `false` (production overlay sets `true`)             |

Production values live in `docker/.env.production` (see `docker/.env.production.example`).
