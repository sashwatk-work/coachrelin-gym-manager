#!/bin/sh
# Runs once when the Postgres data directory is first created.
# Creates a NON-superuser application role so row-level security is enforced.
set -eu

pass="${GYM_DB_PASSWORD:-gym}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=gym_pass="$pass" <<'SQL'
CREATE ROLE gym LOGIN PASSWORD :'gym_pass' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
ALTER DATABASE gym OWNER TO gym;
GRANT ALL ON SCHEMA public TO gym;
SQL
