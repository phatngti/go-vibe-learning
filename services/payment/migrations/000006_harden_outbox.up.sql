ALTER TABLE public.outbox_events ADD COLUMN IF NOT EXISTS dead_at TIMESTAMPTZ;
DROP INDEX IF EXISTS public.idx_outbox_events_claim;
CREATE INDEX IF NOT EXISTS idx_outbox_events_pending
    ON public.outbox_events (available_at, created_at, id)
    WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_outbox_events_processing
    ON public.outbox_events (locked_at)
    WHERE status = 'PROCESSING';
CREATE INDEX IF NOT EXISTS idx_outbox_events_ordering
    ON public.outbox_events (message_key, created_at, id)
    WHERE status IN ('PENDING', 'PROCESSING', 'DEAD');
CREATE INDEX IF NOT EXISTS idx_outbox_events_retention
    ON public.outbox_events (COALESCE(processed_at, dead_at))
    WHERE status IN ('PROCESSED', 'DEAD');
