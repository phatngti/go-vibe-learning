CREATE TABLE IF NOT EXISTS public.event_consumers (
    consumer_name TEXT PRIMARY KEY,
    last_event_time TIMESTAMPTZ NOT NULL DEFAULT to_timestamp(0),
    last_event_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
