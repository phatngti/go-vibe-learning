ALTER TABLE public.commission_ledgers
    DROP CONSTRAINT IF EXISTS commission_ledgers_money_v2_complete;

ALTER TABLE public.agent_balances
    DROP CONSTRAINT IF EXISTS agent_balances_money_v2_complete;

ALTER TABLE public.commission_ledgers
    DROP COLUMN IF EXISTS currency_exponent,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS commission_amount_minor,
    DROP COLUMN IF EXISTS transaction_amount_minor;

ALTER TABLE public.agent_balances
    DROP COLUMN IF EXISTS currency_exponent,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS credit_limit_minor,
    DROP COLUMN IF EXISTS active_balance_minor;

DROP TABLE IF EXISTS public.money_migration_quarantine;
