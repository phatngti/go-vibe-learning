ALTER TABLE public.agent_configs
    ADD COLUMN IF NOT EXISTS callback_challenge_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS callback_challenge_digest TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS callback_challenge_issued_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS callback_challenge_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS callback_challenge_used_at TIMESTAMPTZ;

-- A static domain allowlist is an egress policy, not ownership proof. Existing
-- allowlist-derived activation must complete the one-time challenge again.
UPDATE public.agent_configs
SET callback_status = 'PENDING_VERIFICATION', callback_verified_at = NULL
WHERE callback_status = 'VERIFIED'
  AND callback_challenge_used_at IS NULL;

ALTER TABLE public.agent_configs
    DROP CONSTRAINT IF EXISTS agent_configs_callback_challenge_state_check;

ALTER TABLE public.agent_configs
    ADD CONSTRAINT agent_configs_callback_challenge_state_check CHECK (
        (
            callback_challenge_id = ''
            AND callback_challenge_digest = ''
            AND callback_challenge_issued_at IS NULL
            AND callback_challenge_expires_at IS NULL
            AND callback_challenge_used_at IS NULL
        )
        OR (
            btrim(callback_challenge_id) <> ''
            AND btrim(callback_challenge_digest) <> ''
            AND callback_challenge_issued_at IS NOT NULL
            AND callback_challenge_expires_at > callback_challenge_issued_at
            AND (callback_challenge_used_at IS NULL OR callback_challenge_used_at >= callback_challenge_issued_at)
        )
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_configs_callback_challenge
    ON public.agent_configs (callback_challenge_id)
    WHERE callback_challenge_id <> '';

COMMENT ON COLUMN public.agent_configs.callback_challenge_digest IS
    'SHA-256 digest of a one-time ownership token; the plaintext token is never persisted.';
