CREATE TABLE IF NOT EXISTS public.agent_configs (
    agent_id TEXT PRIMARY KEY,
    callback_url TEXT NOT NULL,
    api_key TEXT UNIQUE NOT NULL,
    allowed_ip_s TEXT DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS public.agent_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    agent_id TEXT NOT NULL,
    action TEXT NOT NULL,
    details TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
