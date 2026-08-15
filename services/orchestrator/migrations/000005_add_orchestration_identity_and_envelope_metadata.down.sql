DROP INDEX IF EXISTS public.idx_payment_sagas_orchestration_owner;

ALTER TABLE public.outbox_events
    DROP CONSTRAINT IF EXISTS ck_outbox_orchestration_identity,
    DROP COLUMN IF EXISTS tracestate,
    DROP COLUMN IF EXISTS traceparent,
    DROP COLUMN IF EXISTS orchestration_generation,
    DROP COLUMN IF EXISTS orchestration_owner,
    DROP COLUMN IF EXISTS causation_id,
    DROP COLUMN IF EXISTS correlation_id,
    DROP COLUMN IF EXISTS tenant_id,
    DROP COLUMN IF EXISTS aggregate_version,
    DROP COLUMN IF EXISTS schema_version;

ALTER TABLE public.payment_sagas
    DROP CONSTRAINT IF EXISTS ck_payment_sagas_orchestration_generation,
    DROP COLUMN IF EXISTS orchestration_generation,
    DROP COLUMN IF EXISTS orchestration_owner;
