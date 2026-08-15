ALTER TABLE public.payment_sagas
    ADD COLUMN IF NOT EXISTS resume_state TEXT NOT NULL DEFAULT '';

DROP INDEX IF EXISTS public.idx_payment_sagas_retry;
CREATE INDEX IF NOT EXISTS idx_payment_sagas_retry
    ON public.payment_sagas (next_retry_at, transaction_id)
    WHERE state IN ('WAITING_PAYMENT', 'WAITING_AFFILIATE', 'WAITING_SETTLEMENT', 'WAITING_CALLBACK');

CREATE INDEX IF NOT EXISTS idx_payment_sagas_manual_review
    ON public.payment_sagas (updated_at, transaction_id)
    WHERE state = 'NEEDS_MANUAL_REVIEW';
