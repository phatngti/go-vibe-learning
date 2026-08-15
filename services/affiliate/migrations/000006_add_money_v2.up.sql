ALTER TABLE public.agent_metrics
    ADD COLUMN IF NOT EXISTS accumulated_volume_minor BIGINT,
    ADD COLUMN IF NOT EXISTS currency TEXT,
    ADD COLUMN IF NOT EXISTS currency_exponent SMALLINT;

CREATE TABLE IF NOT EXISTS public.money_migration_quarantine (
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    legacy_amount TEXT NOT NULL,
    reason TEXT NOT NULL,
    quarantined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_type, entity_id)
);

-- Existing affiliate totals do not contain currency provenance. Guessing a
-- currency would silently corrupt rank calculations, so non-zero rows are
-- quarantined in bounded batches for an operator-approved backfill. Zero rows
-- are initialized atomically when their first Money v2 command arrives.
DO $$
DECLARE
    inserted_rows INTEGER;
BEGIN
    LOOP
        WITH batch AS (
            SELECT metrics.agent_id, metrics.accumulated_volume::TEXT AS legacy_amount
            FROM public.agent_metrics AS metrics
            WHERE metrics.accumulated_volume <> 0
              AND metrics.accumulated_volume_minor IS NULL
              AND NOT EXISTS (
                  SELECT 1
                  FROM public.money_migration_quarantine AS quarantine
                  WHERE quarantine.entity_type = 'agent_metric'
                    AND quarantine.entity_id = metrics.agent_id
              )
            ORDER BY metrics.agent_id
            LIMIT 1000
        )
        INSERT INTO public.money_migration_quarantine (entity_type, entity_id, legacy_amount, reason)
        SELECT 'agent_metric', agent_id, legacy_amount, 'MISSING_CURRENCY_PROVENANCE'
        FROM batch
        ON CONFLICT (entity_type, entity_id) DO NOTHING;

        GET DIAGNOSTICS inserted_rows = ROW_COUNT;
        EXIT WHEN inserted_rows = 0;
    END LOOP;
END $$;

ALTER TABLE public.agent_metrics
    DROP CONSTRAINT IF EXISTS agent_metrics_money_v2_complete;

ALTER TABLE public.agent_metrics
    ADD CONSTRAINT agent_metrics_money_v2_complete CHECK (
        (accumulated_volume_minor IS NULL AND currency IS NULL AND currency_exponent IS NULL)
        OR
        (accumulated_volume_minor IS NOT NULL AND currency IS NOT NULL AND currency_exponent IS NOT NULL
            AND accumulated_volume_minor >= 0
            AND currency_exponent BETWEEN 0 AND 3)
    );

COMMENT ON COLUMN public.agent_metrics.accumulated_volume_minor IS
    'Authoritative Money v2 total. Legacy accumulated_volume is compatibility-only after backfill.';
