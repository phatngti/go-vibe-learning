CREATE TABLE IF NOT EXISTS public.money_conversion_quarantine (
    transaction_id UUID PRIMARY KEY,
    legacy_amount NUMERIC(15,2) NOT NULL,
    currency TEXT NOT NULL,
    reason TEXT NOT NULL,
    quarantined_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE public.transactions
    ADD COLUMN IF NOT EXISTS amount_minor BIGINT,
    ADD COLUMN IF NOT EXISTS currency_exponent SMALLINT,
    ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS callback_endpoint_id TEXT NOT NULL DEFAULT '';

UPDATE public.transactions
SET currency = upper(btrim(currency));

INSERT INTO public.money_conversion_quarantine (transaction_id, legacy_amount, currency, reason)
SELECT id, amount, currency,
       CASE
           WHEN currency NOT IN ('AUD','CAD','CHF','CNY','EUR','GBP','HKD','IDR','INR','JPY','KRW','MYR','NZD','PHP','SGD','THB','USD','VND')
               THEN 'unsupported ISO 4217 currency'
           ELSE 'legacy amount has precision below the currency minor unit'
       END
FROM public.transactions
WHERE currency NOT IN ('AUD','CAD','CHF','CNY','EUR','GBP','HKD','IDR','INR','JPY','KRW','MYR','NZD','PHP','SGD','THB','USD','VND')
   OR (currency IN ('JPY','KRW','VND') AND amount <> trunc(amount))
ON CONFLICT (transaction_id) DO NOTHING;

UPDATE public.transactions
SET amount_minor = CASE
        WHEN currency IN ('JPY','KRW','VND') THEN amount::BIGINT
        ELSE (amount * 100)::BIGINT
    END,
    currency_exponent = CASE
        WHEN currency IN ('JPY','KRW','VND') THEN 0
        ELSE 2
    END
WHERE id NOT IN (SELECT transaction_id FROM public.money_conversion_quarantine)
  AND amount_minor IS NULL;

ALTER TABLE public.transactions
    DROP CONSTRAINT IF EXISTS chk_transactions_money_pair,
    ADD CONSTRAINT chk_transactions_money_pair CHECK (
        (amount_minor IS NULL AND currency_exponent IS NULL)
        OR (amount_minor IS NOT NULL AND currency_exponent IS NOT NULL)
    ),
    DROP CONSTRAINT IF EXISTS chk_transactions_currency_exponent,
    ADD CONSTRAINT chk_transactions_currency_exponent CHECK (
        currency_exponent IS NULL OR currency_exponent BETWEEN 0 AND 3
    ),
    DROP CONSTRAINT IF EXISTS chk_transactions_positive_minor_amount,
    ADD CONSTRAINT chk_transactions_positive_minor_amount CHECK (
        amount_minor IS NULL OR amount_minor > 0
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_transactions_provider_reference
    ON public.transactions (provider, provider_reference)
    WHERE provider <> '' AND provider_reference <> '';

CREATE INDEX IF NOT EXISTS idx_transactions_provider_status
    ON public.transactions (provider, provider_status, updated_at)
    WHERE provider <> '';

