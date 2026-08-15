UPDATE public.callback_deliveries
SET status = 'TERMINAL_FAILED'
WHERE status = 'FAILED';

ALTER TABLE public.callback_deliveries
    ADD COLUMN IF NOT EXISTS payload BYTEA NOT NULL DEFAULT ''::bytea,
    ADD COLUMN IF NOT EXISTS generation_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS lease_owner TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS lease_token BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS replay_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS outcome_generation BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS terminal_at TIMESTAMPTZ;

UPDATE public.callback_deliveries
SET next_attempt_at = COALESCE(next_attempt_at, updated_at, now())
WHERE status IN ('PENDING', 'RETRY_SCHEDULED');

ALTER TABLE public.callback_deliveries
    DROP CONSTRAINT IF EXISTS callback_deliveries_status_check;

ALTER TABLE public.callback_deliveries
    ADD CONSTRAINT callback_deliveries_status_check
    CHECK (status IN ('PENDING', 'IN_FLIGHT', 'RETRY_SCHEDULED', 'DELIVERED', 'SKIPPED', 'TERMINAL_FAILED'));

CREATE UNIQUE INDEX IF NOT EXISTS uq_callback_deliveries_event
    ON public.callback_deliveries (event_id)
    WHERE event_id <> '';

CREATE INDEX IF NOT EXISTS idx_callback_deliveries_ready
    ON public.callback_deliveries (next_attempt_at, created_at)
    WHERE status IN ('PENDING', 'RETRY_SCHEDULED');

CREATE INDEX IF NOT EXISTS idx_callback_deliveries_expired_lease
    ON public.callback_deliveries (lease_expires_at)
    WHERE status = 'IN_FLIGHT';

CREATE TABLE IF NOT EXISTS public.callback_dead_letters (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL,
    transaction_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    outcome_generation BIGINT NOT NULL,
    attempts INTEGER NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    replayed_at TIMESTAMPTZ,
    replayed_by TEXT NOT NULL DEFAULT '',
    replay_reason TEXT NOT NULL DEFAULT '',
    CONSTRAINT callback_dead_letters_status_check CHECK (status IN ('OPEN', 'REPLAYED')),
    CONSTRAINT uq_callback_dead_letters_generation UNIQUE (event_id, outcome_generation)
);

CREATE INDEX IF NOT EXISTS idx_callback_dead_letters_tenant_status_created
    ON public.callback_dead_letters (tenant_id, status, created_at DESC);

COMMENT ON TABLE public.callback_dead_letters IS
    'Auditable terminal callback generations. Replay preserves the original event identity and increments outcome_generation.';
