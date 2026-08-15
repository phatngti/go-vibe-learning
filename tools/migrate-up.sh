#!/bin/sh
set -eu

service="${1:-${SERVICE_NAME:-}}"
case "$service" in
  user|payment|affiliate|settlement|agent|orchestrator) ;;
  *)
    echo "migrate-up: supported SERVICE_NAME is required" >&2
    exit 2
    ;;
esac

migrations_dir="/migrations/$service"
if [ ! -d "$migrations_dir" ]; then
  echo "migrate-up: migration directory not found for $service" >&2
  exit 2
fi

psql -v ON_ERROR_STOP=1 -X -c \
  "CREATE TABLE IF NOT EXISTS public.schema_migrations(version TEXT PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())"

for migration in "$migrations_dir"/*.up.sql; do
  [ -f "$migration" ] || continue
  filename="$(basename "$migration")"
  version="${filename%%_*}"
  applied="$(psql -v ON_ERROR_STOP=1 -X -Atc "SELECT 1 FROM public.schema_migrations WHERE version = '$version'")"
  if [ "$applied" = "1" ]; then
    echo "migrate-up: $service $filename already applied"
    continue
  fi
  echo "migrate-up: applying $service $filename"
  psql -v ON_ERROR_STOP=1 -X --single-transaction \
    -f "$migration" \
    -c "INSERT INTO public.schema_migrations(version, name) VALUES ('$version', '$filename')"
done
