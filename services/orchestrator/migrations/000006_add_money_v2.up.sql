ALTER TABLE public.payment_sagas
    ADD COLUMN IF NOT EXISTS amount_minor BIGINT,
    ADD COLUMN IF NOT EXISTS currency_exponent SMALLINT;

CREATE TABLE IF NOT EXISTS public.money_migration_quarantine (
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    legacy_amount TEXT NOT NULL,
    currency TEXT NOT NULL,
    reason TEXT NOT NULL,
    quarantined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (entity_type, entity_id)
);

-- Orchestrator rows already own currency, so representable values can be
-- backfilled safely. Each transaction updates at most 1000 rows to keep locks
-- bounded on large tables.
DO $$
DECLARE
    updated_rows INTEGER;
BEGIN
    LOOP
        WITH batch AS (
            SELECT saga.transaction_id,
                   CASE
                       WHEN upper(saga.currency) IN ('JPY', 'KRW', 'VND') THEN 0
                       WHEN upper(saga.currency) IN ('AUD', 'CAD', 'CHF', 'CNY', 'EUR', 'GBP', 'HKD', 'IDR', 'INR', 'MYR', 'NZD', 'PHP', 'SGD', 'THB', 'USD') THEN 2
                   END::SMALLINT AS exponent,
                   CASE
                       WHEN upper(saga.currency) IN ('JPY', 'KRW', 'VND') THEN saga.amount
                       ELSE saga.amount * 100
                   END::BIGINT AS minor
            FROM public.payment_sagas AS saga
            WHERE saga.amount_minor IS NULL
              AND upper(saga.currency) IN ('AUD', 'CAD', 'CHF', 'CNY', 'EUR', 'GBP', 'HKD', 'IDR', 'INR', 'JPY', 'KRW', 'MYR', 'NZD', 'PHP', 'SGD', 'THB', 'USD', 'VND')
              AND (
                  (upper(saga.currency) IN ('JPY', 'KRW', 'VND') AND saga.amount = trunc(saga.amount))
                  OR
                  (upper(saga.currency) NOT IN ('JPY', 'KRW', 'VND') AND saga.amount * 100 = trunc(saga.amount * 100))
              )
            ORDER BY saga.transaction_id
            LIMIT 1000
        )
        UPDATE public.payment_sagas AS saga
        SET amount_minor = batch.minor,
            currency = upper(saga.currency),
            currency_exponent = batch.exponent
        FROM batch
        WHERE saga.transaction_id = batch.transaction_id;

        GET DIAGNOSTICS updated_rows = ROW_COUNT;
        EXIT WHEN updated_rows = 0;
    END LOOP;
END $$;

INSERT INTO public.money_migration_quarantine (entity_type, entity_id, legacy_amount, currency, reason)
SELECT 'payment_saga', saga.transaction_id, saga.amount::TEXT, saga.currency,
       CASE
           WHEN upper(saga.currency) NOT IN ('AUD', 'CAD', 'CHF', 'CNY', 'EUR', 'GBP', 'HKD', 'IDR', 'INR', 'JPY', 'KRW', 'MYR', 'NZD', 'PHP', 'SGD', 'THB', 'USD', 'VND')
               THEN 'UNSUPPORTED_CURRENCY'
           ELSE 'NON_REPRESENTABLE_MINOR_UNITS'
       END
FROM public.payment_sagas AS saga
WHERE saga.amount_minor IS NULL
ON CONFLICT (entity_type, entity_id) DO UPDATE
SET legacy_amount = EXCLUDED.legacy_amount,
    currency = EXCLUDED.currency,
    reason = EXCLUDED.reason,
    quarantined_at = now();

ALTER TABLE public.payment_sagas
    DROP CONSTRAINT IF EXISTS payment_sagas_money_v2_complete;

ALTER TABLE public.payment_sagas
    ADD CONSTRAINT payment_sagas_money_v2_complete CHECK (
        (amount_minor IS NULL AND currency_exponent IS NULL)
        OR
        (amount_minor IS NOT NULL AND currency_exponent IS NOT NULL AND currency_exponent BETWEEN 0 AND 3)
    );

COMMENT ON COLUMN public.payment_sagas.amount_minor IS
    'Authoritative Money v2 amount. Legacy amount is compatibility-only after backfill.';
