DROP INDEX IF EXISTS public.idx_transactions_provider_status;
DROP INDEX IF EXISTS public.uq_transactions_provider_reference;

ALTER TABLE public.transactions
    DROP CONSTRAINT IF EXISTS chk_transactions_positive_minor_amount,
    DROP CONSTRAINT IF EXISTS chk_transactions_currency_exponent,
    DROP CONSTRAINT IF EXISTS chk_transactions_money_pair,
    DROP COLUMN IF EXISTS callback_endpoint_id,
    DROP COLUMN IF EXISTS provider_status,
    DROP COLUMN IF EXISTS provider,
    DROP COLUMN IF EXISTS currency_exponent,
    DROP COLUMN IF EXISTS amount_minor;

DROP TABLE IF EXISTS public.money_conversion_quarantine;

