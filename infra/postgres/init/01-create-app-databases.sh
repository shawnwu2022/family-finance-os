#!/bin/sh
set -eu

: "${FINANCE_DB_NAME:?FINANCE_DB_NAME is required}"
: "${FINANCE_DB_USER:?FINANCE_DB_USER is required}"
: "${FINANCE_DB_PASSWORD:?FINANCE_DB_PASSWORD is required}"
: "${EBK_DB_NAME:?EBK_DB_NAME is required}"
: "${EBK_DB_USER:?EBK_DB_USER is required}"
: "${EBK_DB_PASSWORD:?EBK_DB_PASSWORD is required}"

psql -v ON_ERROR_STOP=1 \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" \
  -v finance_db="$FINANCE_DB_NAME" \
  -v finance_user="$FINANCE_DB_USER" \
  -v finance_pass="$FINANCE_DB_PASSWORD" \
  -v ebk_db="$EBK_DB_NAME" \
  -v ebk_user="$EBK_DB_USER" \
  -v ebk_pass="$EBK_DB_PASSWORD" <<'EOSQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'finance_user', :'finance_pass')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'finance_user')
\gexec

SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'ebk_user', :'ebk_pass')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'ebk_user')
\gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'finance_db', :'finance_user')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'finance_db')
\gexec

SELECT format('CREATE DATABASE %I OWNER %I', :'ebk_db', :'ebk_user')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'ebk_db')
\gexec
EOSQL
