DROP INDEX IF EXISTS public.uq_transactions_agent_request_idempotency;

ALTER TABLE public.transactions
    DROP COLUMN IF EXISTS request_fingerprint,
    DROP COLUMN IF EXISTS request_idempotency_key;
