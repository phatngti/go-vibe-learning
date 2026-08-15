DROP INDEX IF EXISTS public.idx_callback_dead_letters_tenant_status_created;
DROP TABLE IF EXISTS public.callback_dead_letters;

DROP INDEX IF EXISTS public.idx_callback_deliveries_expired_lease;
DROP INDEX IF EXISTS public.idx_callback_deliveries_ready;
DROP INDEX IF EXISTS public.uq_callback_deliveries_event;

ALTER TABLE public.callback_deliveries
    DROP CONSTRAINT IF EXISTS callback_deliveries_status_check;

UPDATE public.callback_deliveries
SET status = 'FAILED'
WHERE status = 'TERMINAL_FAILED';

ALTER TABLE public.callback_deliveries
    DROP COLUMN IF EXISTS terminal_at,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS outcome_generation,
    DROP COLUMN IF EXISTS replay_count,
    DROP COLUMN IF EXISTS lease_expires_at,
    DROP COLUMN IF EXISTS lease_token,
    DROP COLUMN IF EXISTS lease_owner,
    DROP COLUMN IF EXISTS last_attempt_at,
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS generation_attempts,
    DROP COLUMN IF EXISTS payload;
