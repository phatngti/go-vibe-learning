CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS public.user_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_provider TEXT NOT NULL DEFAULT 'keycloak',
    identity_provider_user_id TEXT NOT NULL,
    username TEXT NOT NULL,
    email TEXT,
    first_name TEXT,
    last_name TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    identity_created_at TIMESTAMPTZ,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_identities_provider_user_unique
        UNIQUE (identity_provider, identity_provider_user_id),
    CONSTRAINT user_identities_provider_not_blank
        CHECK (btrim(identity_provider) <> ''),
    CONSTRAINT user_identities_provider_user_id_not_blank
        CHECK (btrim(identity_provider_user_id) <> ''),
    CONSTRAINT user_identities_username_not_blank
        CHECK (btrim(username) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS user_identities_provider_username_unique
    ON public.user_identities (identity_provider, lower(username));

CREATE UNIQUE INDEX IF NOT EXISTS user_identities_provider_email_unique
    ON public.user_identities (identity_provider, lower(email))
    WHERE email IS NOT NULL AND btrim(email) <> '';

CREATE INDEX IF NOT EXISTS user_identities_last_synced_at_idx
    ON public.user_identities (last_synced_at);
