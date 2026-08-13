-- Append-only domain event log for event sourcing.
CREATE TABLE IF NOT EXISTS public.domain_events (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT        NOT NULL,
    aggregate_id   TEXT        NOT NULL,
    event_type     TEXT        NOT NULL,
    payload        TEXT        NOT NULL,
    version        BIGINT      NOT NULL,
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_domain_events_version UNIQUE (aggregate_type, aggregate_id, version)
);

CREATE INDEX IF NOT EXISTS idx_domain_events_aggregate
    ON public.domain_events (aggregate_type, aggregate_id, version);
