CREATE TABLE IF NOT EXISTS public.provider_webhook_events (
    provider TEXT NOT NULL,
    event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    transaction_id TEXT NOT NULL,
    provider_reference TEXT NOT NULL,
    provider_status TEXT NOT NULL,
    payload_sha256 TEXT NOT NULL,
    processing_status TEXT NOT NULL DEFAULT 'RECEIVED',
    last_error TEXT NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    PRIMARY KEY (provider, event_id),
    CONSTRAINT chk_provider_webhook_processing_status
        CHECK (processing_status IN ('RECEIVED', 'PROCESSED', 'IGNORED'))
);

CREATE INDEX IF NOT EXISTS idx_provider_webhook_transaction
    ON public.provider_webhook_events (transaction_id, received_at DESC);

CREATE INDEX IF NOT EXISTS idx_provider_webhook_received
    ON public.provider_webhook_events (processing_status, received_at)
    WHERE processing_status = 'RECEIVED';

