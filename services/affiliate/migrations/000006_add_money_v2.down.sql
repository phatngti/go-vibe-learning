ALTER TABLE public.agent_metrics
    DROP CONSTRAINT IF EXISTS agent_metrics_money_v2_complete;

ALTER TABLE public.agent_metrics
    DROP COLUMN IF EXISTS currency_exponent,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS accumulated_volume_minor;

DROP TABLE IF EXISTS public.money_migration_quarantine;
