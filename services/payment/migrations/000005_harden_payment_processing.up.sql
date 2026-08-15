ALTER TABLE public.transactions
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_reference TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS processing_started_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS uq_transactions_idempotency_key
    ON public.transactions (idempotency_key)
    WHERE idempotency_key <> '';

CREATE INDEX IF NOT EXISTS idx_transactions_processing_recovery
    ON public.transactions (status, processing_started_at)
    WHERE status IN ('PROCESSING', 'UNKNOWN');
