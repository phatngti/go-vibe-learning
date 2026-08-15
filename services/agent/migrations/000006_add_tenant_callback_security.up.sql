ALTER TABLE public.agent_configs
    ADD COLUMN IF NOT EXISTS tenant_id TEXT,
    ADD COLUMN IF NOT EXISTS callback_host TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS callback_status TEXT NOT NULL DEFAULT 'PENDING_VERIFICATION',
    ADD COLUMN IF NOT EXISTS callback_verified_at TIMESTAMPTZ;

ALTER TABLE public.agent_audit_logs
    ADD COLUMN IF NOT EXISTS tenant_id TEXT;

ALTER TABLE public.callback_deliveries
    ADD COLUMN IF NOT EXISTS tenant_id TEXT,
    ADD COLUMN IF NOT EXISTS event_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS key_version TEXT NOT NULL DEFAULT '';

ALTER TABLE public.agent_configs
    DROP CONSTRAINT IF EXISTS agent_configs_callback_status_check;

ALTER TABLE public.agent_configs
    ADD CONSTRAINT agent_configs_callback_status_check
    CHECK (callback_status IN ('PENDING_VERIFICATION', 'VERIFIED'));

ALTER TABLE public.agent_configs
    DROP CONSTRAINT IF EXISTS agent_configs_tenant_not_blank;

ALTER TABLE public.agent_configs
    ADD CONSTRAINT agent_configs_tenant_not_blank
    CHECK (tenant_id IS NULL OR btrim(tenant_id) <> '');

CREATE INDEX IF NOT EXISTS idx_agent_configs_tenant_agent
    ON public.agent_configs (tenant_id, agent_id)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_agent_audit_logs_tenant_created
    ON public.agent_audit_logs (tenant_id, created_at DESC)
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_callback_deliveries_tenant_event
    ON public.callback_deliveries (tenant_id, event_id)
    WHERE tenant_id IS NOT NULL AND event_id <> '';

COMMENT ON COLUMN public.agent_configs.tenant_id IS
    'Security partition derived from verified identity claims. Existing NULL rows require an explicit ownership backfill before production enforcement.';

COMMENT ON COLUMN public.agent_configs.callback_status IS
    'Only VERIFIED endpoints may receive production callback traffic.';

COMMENT ON COLUMN public.agent_configs.allowed_ips IS
    'Deprecated client input. Callback delivery resolves and validates public addresses at connection time.';
