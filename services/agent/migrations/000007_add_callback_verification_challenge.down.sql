DROP INDEX IF EXISTS public.uq_agent_configs_callback_challenge;

ALTER TABLE public.agent_configs
    DROP CONSTRAINT IF EXISTS agent_configs_callback_challenge_state_check,
    DROP COLUMN IF EXISTS callback_challenge_used_at,
    DROP COLUMN IF EXISTS callback_challenge_expires_at,
    DROP COLUMN IF EXISTS callback_challenge_issued_at,
    DROP COLUMN IF EXISTS callback_challenge_digest,
    DROP COLUMN IF EXISTS callback_challenge_id;
