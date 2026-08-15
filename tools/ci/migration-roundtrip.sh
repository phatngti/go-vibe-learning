#!/usr/bin/env bash

set -euo pipefail

services=(user payment affiliate settlement agent orchestrator)

wait_for_postgres() {
  local attempts=0
  until pg_isready -q; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 30 ]; then
      echo "PostgreSQL did not become ready" >&2
      return 1
    fi
    sleep 1
  done
}

apply_up() {
  local database="$1"
  local directory="$2"
  local file version name
  while IFS= read -r file; do
    version="$(basename "$file" | cut -d_ -f1)"
    name="$(basename "$file" .up.sql)"
    psql -v ON_ERROR_STOP=1 -d "$database" -f "$file"
    psql -v ON_ERROR_STOP=1 -d "$database" -c \
      "INSERT INTO public.schema_migrations(version, name) VALUES ('$version', '$name') ON CONFLICT (version) DO NOTHING"
  done < <(find "$directory" -maxdepth 1 -type f -name '*.up.sql' | sort)
}

apply_down() {
  local database="$1"
  local directory="$2"
  local file version
  while IFS= read -r file; do
    version="$(basename "$file" | cut -d_ -f1)"
    psql -v ON_ERROR_STOP=1 -d "$database" -f "$file"
    psql -v ON_ERROR_STOP=1 -d "$database" -c \
      "DELETE FROM public.schema_migrations WHERE version = '$version'"
  done < <(find "$directory" -maxdepth 1 -type f -name '*.down.sql' | sort -r)
}

wait_for_postgres

for service in "${services[@]}"; do
  database="cqrs_ci_${service}"
  directory="services/${service}/migrations"
  if [ ! -d "$directory" ]; then
    echo "Missing migration directory: $directory" >&2
    exit 1
  fi

  createdb "$database"
  psql -v ON_ERROR_STOP=1 -d "$database" -c \
    "CREATE TABLE public.schema_migrations(version TEXT PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())"

  echo "Applying $service migrations"
  apply_up "$database" "$directory"
  echo "Reverting $service migrations"
  apply_down "$database" "$directory"
  echo "Reapplying $service migrations"
  apply_up "$database" "$directory"
done

echo "All service migrations passed up/down/up validation"
