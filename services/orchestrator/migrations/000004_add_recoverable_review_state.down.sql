DROP INDEX IF EXISTS public.idx_payment_sagas_manual_review;
DROP INDEX IF EXISTS public.idx_payment_sagas_retry;
CREATE INDEX IF NOT EXISTS idx_payment_sagas_retry
    ON public.payment_sagas (state, next_retry_at);
ALTER TABLE public.payment_sagas DROP COLUMN IF EXISTS resume_state;
