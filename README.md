# Coach Relin Fitness

Operations console for **Coach Relin Fitness**. Tracks members, membership plans
and subscriptions, payments, trainers, and personal-training packages and sessions.

This is Relin’s product, not a public gym-manager SaaS. Relin is the owner. Each
location is a tenant with its own members and staff; more gyms can be added later
without mixing data.

- **Backend:** Go 1.25, chi, pgx, PostgreSQL 16 with row-level security per gym
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS, TanStack Query, React Router

## Quick start (local Go + Vite)

Prerequisites: Go 1.25+, Node 20+, Docker.

Copy the local env template once. `docker/.env` is gitignored.

```bash
cp docker/.env.example docker/.env
make db-up        # Postgres on 127.0.0.1:5432
make migrate      # schema + Coach Relin Fitness + Relin’s login
make api          # http://127.0.0.1:8080
make web          # http://127.0.0.1:5173 (proxies /api to :8080)
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

`make db-down` stops the local Compose services. It does not delete the Postgres volume.

## Run everything in Docker

This builds the same images as production and publishes them on loopback only:

```bash
cp docker/.env.example docker/.env
make up           # postgres + backend + frontend
make down
```

| Service    | Address              |
|------------|----------------------|
| Frontend   | http://127.0.0.1:5173 |
| Backend    | http://127.0.0.1:8080 |
| PostgreSQL | 127.0.0.1:5432        |

Do not run `make web` at the same time as `make up`; both bind 5173.

Docker starts the same empty Coach Relin Fitness gym. Use `make seed` if you want sample members.

Compose lives under `docker/`. Direct commands need `-f docker/compose.yaml`.

## Production (droplet)

Production is the local Compose file plus `docker/compose.production.yaml`. Nginx is the only public process. It terminates TLS and proxies `/` to the frontend and `/api/` to the backend over private networks. PostgreSQL has no host port.

```bash
cp docker/.env.production.example docker/.env.production
# Fill domain, secrets, Postgres data dir, and TLS cert paths.
make production-config
make production-up
make production-down    # stops containers; does not delete data
```

Point `CRF_PRODUCTION_DOMAIN` at the droplet (A/AAAA). Open 80 and 443. Mount Let’s Encrypt (or other) files as `CRF_PRODUCTION_TLS_CERTIFICATE` and `CRF_PRODUCTION_TLS_PRIVATE_KEY`.

First login is still `owner@coachrelin.fit` / `password123`. Change that password immediately on a real gym.

`COOKIE_SECURE` is forced on in the production overlay.

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
backend/                 Go API (cmd/api, handlers, store, goose migrations)
frontend/                React app
config/frontend/         Nginx config baked into the frontend image
config/nginx/            TLS reverse-proxy template mounted into production Nginx
docker/compose.yaml      Local Compose (postgres, backend, frontend)
docker/compose.production.yaml
                         Overlay: Nginx edge, private networks, no DB host port
docker/.env.example
docker/.env.production.example
```

Use the root Makefile for container operations. Direct Compose commands require
`-f docker/compose.yaml` because the definitions live under `docker/`.

## Environment

Local API (`make api`) and `docker/.env`:

| Variable        | Default                                                |
|-----------------|--------------------------------------------------------|
| `DATABASE_URL`  | `postgres://gym:gym@127.0.0.1:5432/gym?sslmode=disable` |
| `JWT_SECRET`    | `dev-secret-change-me`                                 |
| `PORT`          | `8080`                                                 |
| `COOKIE_SECURE` | `false` (production overlay sets `true`)               |

The app connects as the non-superuser role `gym` so row-level security is enforced.

Production values live in `docker/.env.production` (never commit that file):

| Variable | Purpose |
|----------|---------|
| `CRF_PRODUCTION_DOMAIN` | Hostname only, e.g. `console.coachrelin.com` |
| `CRF_PRODUCTION_WEB_ORIGIN` | Public origin, e.g. `https://console.coachrelin.com` |
| `CRF_PRODUCTION_POSTGRES_PASSWORD` | Postgres superuser (init / break-glass) |
| `CRF_PRODUCTION_GYM_DB_PASSWORD` | App role `gym`; URL-escape it in `DATABASE_URL` |
| `CRF_PRODUCTION_DATABASE_URL` | `postgres://gym:…@postgres:5432/gym?sslmode=disable` |
| `CRF_PRODUCTION_JWT_SECRET` | Session signing key |
| `CRF_PRODUCTION_POSTGRES_DATA_DIR` | Absolute host dir for Postgres files |
| `CRF_PRODUCTION_TLS_CERTIFICATE` | Absolute path to `fullchain.pem` |
| `CRF_PRODUCTION_TLS_PRIVATE_KEY` | Absolute path to `privkey.pem` |
| `CRF_PRODUCTION_HTTP_PORT` / `HTTPS_PORT` | Host ports, default 80 / 443 |
