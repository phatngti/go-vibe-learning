CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS public.agent_balances (
    agent_id TEXT PRIMARY KEY,
    active_balance NUMERIC(15,2) NOT NULL DEFAULT 0,
    credit_limit NUMERIC(15,2) NOT NULL DEFAULT 1000,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS public.commission_ledgers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id TEXT NOT NULL,
    transaction_id TEXT NOT NULL,
    transaction_amount NUMERIC(15,2) NOT NULL,
    commission_amount NUMERIC(15,2) NOT NULL,
    rank TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
