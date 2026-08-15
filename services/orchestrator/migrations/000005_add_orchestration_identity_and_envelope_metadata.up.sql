ALTER TABLE public.payment_sagas
    ADD COLUMN IF NOT EXISTS orchestration_owner TEXT NOT NULL DEFAULT 'saga:v1',
    ADD COLUMN IF NOT EXISTS orchestration_generation BIGINT NOT NULL DEFAULT 1;

ALTER TABLE public.outbox_events
    ADD COLUMN IF NOT EXISTS schema_version INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS aggregate_version BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS correlation_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS causation_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS orchestration_owner TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS orchestration_generation BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS traceparent TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS tracestate TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'ck_payment_sagas_orchestration_generation'
    ) THEN
        ALTER TABLE public.payment_sagas
            ADD CONSTRAINT ck_payment_sagas_orchestration_generation
            CHECK (orchestration_generation > 0);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'ck_outbox_orchestration_identity'
    ) THEN
        ALTER TABLE public.outbox_events
            ADD CONSTRAINT ck_outbox_orchestration_identity
            CHECK (
                (orchestration_owner = '' AND orchestration_generation = 0)
                OR (orchestration_owner <> '' AND orchestration_generation > 0)
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_payment_sagas_orchestration_owner
    ON public.payment_sagas (orchestration_owner, orchestration_generation, updated_at);
