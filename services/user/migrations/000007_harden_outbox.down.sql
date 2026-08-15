DROP INDEX IF EXISTS public.idx_outbox_events_retention;
DROP INDEX IF EXISTS public.idx_outbox_events_ordering;
DROP INDEX IF EXISTS public.idx_outbox_events_processing;
DROP INDEX IF EXISTS public.idx_outbox_events_pending;
CREATE INDEX IF NOT EXISTS idx_outbox_events_claim
    ON public.outbox_events (status, available_at, created_at);
ALTER TABLE public.outbox_events DROP COLUMN IF EXISTS dead_at;

