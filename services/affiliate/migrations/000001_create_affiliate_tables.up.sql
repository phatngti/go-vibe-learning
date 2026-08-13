CREATE TABLE IF NOT EXISTS public.referrals (
    referrer_id TEXT NOT NULL,
    referee_id TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS public.agent_metrics (
    agent_id TEXT PRIMARY KEY,
    accumulated_volume NUMERIC(15,2) NOT NULL DEFAULT 0,
    referral_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS public.affiliate_levels (
    agent_id TEXT PRIMARY KEY,
    current_rank TEXT NOT NULL DEFAULT 'BRONZE',
    promoted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
