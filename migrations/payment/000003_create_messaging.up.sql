ALTER TABLE public.outbox_events
    ADD COLUMN IF NOT EXISTS message_id TEXT,
    ADD COLUMN IF NOT EXISTS topic TEXT,
    ADD COLUMN IF NOT EXISTS message_key TEXT,
    ADD COLUMN IF NOT EXISTS message_type TEXT,
    ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS locked_by TEXT NOT NULL DEFAULT '';

UPDATE public.outbox_events
SET message_id = COALESCE(message_id, id::text),
    topic = COALESCE(topic, 'payments.events'),
    message_key = COALESCE(message_key, aggregate_id);

DO $migration$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'outbox_events' AND column_name = 'event_type'
    ) THEN
        UPDATE public.outbox_events
        SET message_type = COALESCE(message_type, event_type);
    END IF;
END
$migration$;

ALTER TABLE public.outbox_events
    ALTER COLUMN message_id SET NOT NULL,
    ALTER COLUMN topic SET NOT NULL,
    ALTER COLUMN message_key SET NOT NULL,
    ALTER COLUMN message_type SET NOT NULL;

ALTER TABLE public.outbox_events
    DROP COLUMN IF EXISTS event_type;

CREATE UNIQUE INDEX IF NOT EXISTS uq_outbox_events_message_id
    ON public.outbox_events (message_id);
CREATE INDEX IF NOT EXISTS idx_outbox_events_claim
    ON public.outbox_events (status, available_at, created_at);
