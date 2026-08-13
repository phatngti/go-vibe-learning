CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_configs' AND column_name = 'allowed_ip_s'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_configs' AND column_name = 'allowed_ips'
    ) THEN
        ALTER TABLE public.agent_configs RENAME COLUMN allowed_ip_s TO allowed_ips;
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS public.callback_deliveries (
    transaction_id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    callback_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS public.inbox_messages (
    message_id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    message_type TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS public.outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id TEXT NOT NULL UNIQUE,
    topic TEXT NOT NULL,
    message_key TEXT NOT NULL,
    message_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    locked_by TEXT NOT NULL DEFAULT '',
    error_log TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_claim
    ON public.outbox_events (status, available_at, created_at);
