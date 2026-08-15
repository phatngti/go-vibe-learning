DROP INDEX IF EXISTS public.idx_inbox_messages_retention;
ALTER TABLE public.inbox_messages DROP CONSTRAINT IF EXISTS inbox_messages_pkey;
ALTER TABLE public.inbox_messages ADD CONSTRAINT inbox_messages_pkey PRIMARY KEY (message_id);
ALTER TABLE public.inbox_messages DROP COLUMN IF EXISTS consumer_name;
