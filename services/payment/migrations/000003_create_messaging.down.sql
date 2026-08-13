DROP INDEX IF EXISTS public.idx_outbox_events_claim;
DROP INDEX IF EXISTS public.uq_outbox_events_message_id;
ALTER TABLE public.outbox_events
    ADD COLUMN IF NOT EXISTS event_type TEXT;
UPDATE public.outbox_events
SET event_type = COALESCE(event_type, message_type);
ALTER TABLE public.outbox_events
    ALTER COLUMN event_type SET NOT NULL;
ALTER TABLE public.outbox_events
    DROP COLUMN IF EXISTS locked_by,
    DROP COLUMN IF EXISTS locked_at,
    DROP COLUMN IF EXISTS available_at,
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS message_type,
    DROP COLUMN IF EXISTS message_key,
    DROP COLUMN IF EXISTS topic,
    DROP COLUMN IF EXISTS message_id;
