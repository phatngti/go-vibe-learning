DROP INDEX IF EXISTS public.idx_transactions_processing_recovery;
DROP INDEX IF EXISTS public.uq_transactions_idempotency_key;

ALTER TABLE public.transactions
    DROP COLUMN IF EXISTS processing_started_at,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS provider_reference,
    DROP COLUMN IF EXISTS idempotency_key;
