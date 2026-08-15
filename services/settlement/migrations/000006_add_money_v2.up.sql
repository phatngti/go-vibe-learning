ALTER TABLE public.agent_balances
    ADD COLUMN IF NOT EXISTS active_balance_minor BIGINT,
    ADD COLUMN IF NOT EXISTS credit_limit_minor BIGINT,
    ADD COLUMN IF NOT EXISTS currency TEXT,
    ADD COLUMN IF NOT EXISTS currency_exponent SMALLINT;

ALTER TABLE public.commission_ledgers
    ADD COLUMN IF NOT EXISTS transaction_amount_minor BIGINT,
    ADD COLUMN IF NOT EXISTS commission_amount_minor BIGINT,
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

-- Settlement v1 rows do not contain currency provenance. Financial balances
-- and ledgers must not be guessed into a currency. Put them in a review queue
-- in bounded batches; new or zero/default balances are initialized by the
-- first authoritative Money v2 command.
DO $$
DECLARE
    inserted_rows INTEGER;
BEGIN
    LOOP
        WITH batch AS (
            SELECT balances.agent_id,
                   concat('active=', balances.active_balance::TEXT, ',credit=', balances.credit_limit::TEXT) AS legacy_amount
            FROM public.agent_balances AS balances
            WHERE (balances.active_balance <> 0 OR balances.credit_limit <> 1000)
              AND balances.active_balance_minor IS NULL
              AND NOT EXISTS (
                  SELECT 1 FROM public.money_migration_quarantine AS quarantine
                  WHERE quarantine.entity_type = 'agent_balance'
                    AND quarantine.entity_id = balances.agent_id
              )
            ORDER BY balances.agent_id
            LIMIT 1000
        )
        INSERT INTO public.money_migration_quarantine (entity_type, entity_id, legacy_amount, reason)
        SELECT 'agent_balance', agent_id, legacy_amount, 'MISSING_CURRENCY_PROVENANCE'
        FROM batch
        ON CONFLICT (entity_type, entity_id) DO NOTHING;

        GET DIAGNOSTICS inserted_rows = ROW_COUNT;
        EXIT WHEN inserted_rows = 0;
    END LOOP;

    LOOP
        WITH batch AS (
            SELECT ledgers.id::TEXT AS ledger_id,
                   concat('transaction=', ledgers.transaction_amount::TEXT, ',commission=', ledgers.commission_amount::TEXT) AS legacy_amount
            FROM public.commission_ledgers AS ledgers
            WHERE ledgers.transaction_amount_minor IS NULL
              AND NOT EXISTS (
                  SELECT 1 FROM public.money_migration_quarantine AS quarantine
                  WHERE quarantine.entity_type = 'commission_ledger'
                    AND quarantine.entity_id = ledgers.id::TEXT
              )
            ORDER BY ledgers.id
            LIMIT 1000
        )
        INSERT INTO public.money_migration_quarantine (entity_type, entity_id, legacy_amount, reason)
        SELECT 'commission_ledger', ledger_id, legacy_amount, 'MISSING_CURRENCY_PROVENANCE'
        FROM batch
        ON CONFLICT (entity_type, entity_id) DO NOTHING;

        GET DIAGNOSTICS inserted_rows = ROW_COUNT;
        EXIT WHEN inserted_rows = 0;
    END LOOP;
END $$;

ALTER TABLE public.agent_balances
    DROP CONSTRAINT IF EXISTS agent_balances_money_v2_complete;

ALTER TABLE public.agent_balances
    ADD CONSTRAINT agent_balances_money_v2_complete CHECK (
        (active_balance_minor IS NULL AND credit_limit_minor IS NULL AND currency IS NULL AND currency_exponent IS NULL)
        OR
        (active_balance_minor IS NOT NULL AND credit_limit_minor IS NOT NULL AND currency IS NOT NULL AND currency_exponent IS NOT NULL
            AND credit_limit_minor >= 0
            AND currency_exponent BETWEEN 0 AND 3)
    );

ALTER TABLE public.commission_ledgers
    DROP CONSTRAINT IF EXISTS commission_ledgers_money_v2_complete;

ALTER TABLE public.commission_ledgers
    ADD CONSTRAINT commission_ledgers_money_v2_complete CHECK (
        (transaction_amount_minor IS NULL AND commission_amount_minor IS NULL AND currency IS NULL AND currency_exponent IS NULL)
        OR
        (transaction_amount_minor IS NOT NULL AND commission_amount_minor IS NOT NULL AND currency IS NOT NULL AND currency_exponent IS NOT NULL
            AND transaction_amount_minor > 0
            AND commission_amount_minor >= 0
            AND currency_exponent BETWEEN 0 AND 3)
    );

COMMENT ON COLUMN public.commission_ledgers.commission_amount_minor IS
    'Authoritative checked basis-point result. Legacy commission_amount is compatibility-only after backfill.';
