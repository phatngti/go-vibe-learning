DROP INDEX IF EXISTS public.uq_callback_deliveries_tenant_event;
DROP INDEX IF EXISTS public.idx_agent_audit_logs_tenant_created;
DROP INDEX IF EXISTS public.idx_agent_configs_tenant_agent;

ALTER TABLE public.callback_deliveries
    DROP COLUMN IF EXISTS key_version,
    DROP COLUMN IF EXISTS event_id,
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE public.agent_audit_logs
    DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE public.agent_configs
    DROP CONSTRAINT IF EXISTS agent_configs_tenant_not_blank,
    DROP CONSTRAINT IF EXISTS agent_configs_callback_status_check,
    DROP COLUMN IF EXISTS callback_verified_at,
    DROP COLUMN IF EXISTS callback_status,
    DROP COLUMN IF EXISTS callback_host,
    DROP COLUMN IF EXISTS tenant_id;
