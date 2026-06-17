DROP TABLE IF EXISTS public.outbox_events;
DROP TABLE IF EXISTS public.inbox_messages;
DROP TABLE IF EXISTS public.callback_deliveries;
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_configs' AND column_name = 'allowed_ips'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'agent_configs' AND column_name = 'allowed_ip_s'
    ) THEN
        ALTER TABLE public.agent_configs RENAME COLUMN allowed_ips TO allowed_ip_s;
    END IF;
END
$$;
