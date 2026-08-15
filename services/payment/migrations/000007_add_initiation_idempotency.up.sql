ALTER TABLE public.transactions
    ADD COLUMN IF NOT EXISTS request_idempotency_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS request_fingerprint TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_transactions_agent_request_idempotency
    ON public.transactions (agent_id, request_idempotency_key)
    WHERE request_idempotency_key <> '';

COMMENT ON COLUMN public.transactions.request_idempotency_key IS
    'Merchant/order request identity scoped by agent_id; separate from the PSP charge-attempt key.';
COMMENT ON COLUMN public.transactions.request_fingerprint IS
    'SHA-256 of canonical initiation fields used to reject same-key/different-payload requests.';
