CREATE TABLE IF NOT EXISTS public.user_sync_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deduplication_key TEXT NOT NULL UNIQUE,
    kind TEXT NOT NULL,
    identity_provider TEXT NOT NULL DEFAULT 'keycloak',
    provider_user_id TEXT NOT NULL DEFAULT '',
    cursor INTEGER NOT NULL DEFAULT 0,
    page_size INTEGER NOT NULL,
    processed_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_token BIGINT NOT NULL DEFAULT 0,
    lease_expires_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    dead_at TIMESTAMPTZ,
    CONSTRAINT user_sync_jobs_kind_check CHECK (kind IN ('SINGLE', 'FULL')),
    CONSTRAINT user_sync_jobs_status_check CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'DEAD')),
    CONSTRAINT user_sync_jobs_provider_check CHECK (btrim(identity_provider) <> ''),
    CONSTRAINT user_sync_jobs_deduplication_key_check CHECK (btrim(deduplication_key) <> ''),
    CONSTRAINT user_sync_jobs_page_size_check CHECK (page_size BETWEEN 1 AND 1000),
    CONSTRAINT user_sync_jobs_cursor_check CHECK (cursor >= 0 AND processed_count >= 0),
    CONSTRAINT user_sync_jobs_attempts_check CHECK (attempts >= 0),
    CONSTRAINT user_sync_jobs_lease_token_check CHECK (lease_token >= 0),
    CONSTRAINT user_sync_jobs_lease_state_check CHECK (
        (status = 'PROCESSING' AND btrim(lease_owner) <> '' AND lease_expires_at IS NOT NULL) OR
        (status <> 'PROCESSING' AND lease_owner = '' AND lease_expires_at IS NULL)
    ),
    CONSTRAINT user_sync_jobs_terminal_time_check CHECK (
        (status = 'COMPLETED' AND completed_at IS NOT NULL AND dead_at IS NULL) OR
        (status = 'DEAD' AND dead_at IS NOT NULL AND completed_at IS NULL) OR
        (status IN ('PENDING', 'PROCESSING') AND completed_at IS NULL AND dead_at IS NULL)
    ),
    CONSTRAINT user_sync_jobs_payload_check CHECK (
        (kind = 'SINGLE' AND btrim(provider_user_id) <> '') OR
        (kind = 'FULL' AND provider_user_id = '')
    )
);

CREATE INDEX IF NOT EXISTS idx_user_sync_jobs_claim
    ON public.user_sync_jobs (available_at, created_at, id)
    WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_user_sync_jobs_expired_lease
    ON public.user_sync_jobs (lease_expires_at, id)
    WHERE status = 'PROCESSING';

CREATE INDEX IF NOT EXISTS idx_user_sync_jobs_retention
    ON public.user_sync_jobs (COALESCE(completed_at, dead_at))
    WHERE status IN ('COMPLETED', 'DEAD');
