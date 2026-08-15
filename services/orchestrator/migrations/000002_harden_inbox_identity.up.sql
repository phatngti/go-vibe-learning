ALTER TABLE public.inbox_messages
    ADD COLUMN IF NOT EXISTS consumer_name TEXT NOT NULL DEFAULT 'legacy';
ALTER TABLE public.inbox_messages DROP CONSTRAINT IF EXISTS inbox_messages_pkey;
ALTER TABLE public.inbox_messages
    ADD CONSTRAINT inbox_messages_pkey PRIMARY KEY (consumer_name, message_id);
CREATE INDEX IF NOT EXISTS idx_inbox_messages_retention
    ON public.inbox_messages (processed_at)
    WHERE status = 'PROCESSED';
