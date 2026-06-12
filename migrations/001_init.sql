CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    target_url TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    custom_domain TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    total_clicks BIGINT NOT NULL DEFAULT 0,
    last_clicked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS links_domain_code_idx ON links (custom_domain, code);
CREATE INDEX IF NOT EXISTS links_created_at_idx ON links (created_at DESC);

CREATE TABLE IF NOT EXISTS click_events (
    id BIGSERIAL PRIMARY KEY,
    link_code TEXT NOT NULL REFERENCES links(code) ON DELETE CASCADE,
    occurred_at TIMESTAMPTZ NOT NULL,
    country TEXT NOT NULL DEFAULT 'Unknown',
    device TEXT NOT NULL DEFAULT 'Unknown',
    referrer_domain TEXT NOT NULL DEFAULT 'Direct',
    user_agent TEXT NOT NULL DEFAULT '',
    ip_hash TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS click_events_link_time_idx ON click_events (link_code, occurred_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS click_events_request_id_uidx
    ON click_events (request_id)
    WHERE request_id <> '';

CREATE TABLE IF NOT EXISTS analytics_hourly (
    link_code TEXT NOT NULL REFERENCES links(code) ON DELETE CASCADE,
    bucket_start TIMESTAMPTZ NOT NULL,
    country TEXT NOT NULL,
    device TEXT NOT NULL,
    referrer_domain TEXT NOT NULL,
    clicks BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (link_code, bucket_start, country, device, referrer_domain)
);

CREATE INDEX IF NOT EXISTS analytics_hourly_link_bucket_idx ON analytics_hourly (link_code, bucket_start DESC);
