ALTER TABLE public.payment_sagas
    DROP CONSTRAINT IF EXISTS payment_sagas_money_v2_complete;

ALTER TABLE public.payment_sagas
    DROP COLUMN IF EXISTS currency_exponent,
    DROP COLUMN IF EXISTS amount_minor;

DROP TABLE IF EXISTS public.money_migration_quarantine;
