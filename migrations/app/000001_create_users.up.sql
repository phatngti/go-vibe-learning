CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS public.users (
    id uuid primary key default gen_random_uuid(),
    username text not null,
    password text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
