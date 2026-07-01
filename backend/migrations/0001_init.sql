-- 0001_init.sql
-- Initial schema for the tracking platform. Covers read_me.md Steps 10, 12, 16, 19.

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto; -- gen_random_uuid()

-- ---------------------------------------------------------------------------
-- Organizations & users (Step 10, Step 12)
-- ---------------------------------------------------------------------------

CREATE TABLE organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL DEFAULT 'admin', -- admin | member, room to grow
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Devices (Step 10, Step 12 device token, Step 16 items 3/7, Step 19)
-- ---------------------------------------------------------------------------

CREATE TABLE devices (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                        TEXT NOT NULL,
    type                        TEXT NOT NULL DEFAULT 'phone', -- phone | hardware
    api_token_hash              TEXT NOT NULL UNIQUE, -- sha256 of the long-lived device token
    api_token_revoked_at        TIMESTAMPTZ,
    speed_limit_kmh             DOUBLE PRECISION,            -- Step 16 item 3, nullable = no limit
    trip_stop_speed_kmh         DOUBLE PRECISION NOT NULL DEFAULT 2,   -- Step 19
    trip_stop_duration_seconds  INTEGER NOT NULL DEFAULT 300,          -- Step 19
    offline_after_seconds       INTEGER NOT NULL DEFAULT 300,          -- Step 16 item 7
    last_position_id            BIGINT, -- denormalized pointer, FK added after positions table exists
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_devices_org_id ON devices(org_id);

-- ---------------------------------------------------------------------------
-- Positions (Step 10) — append-only
-- ---------------------------------------------------------------------------

CREATE TABLE positions (
    id           BIGSERIAL PRIMARY KEY,
    device_id    UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    geom         GEOGRAPHY(POINT, 4326) NOT NULL,
    speed        DOUBLE PRECISION,        -- km/h
    heading      DOUBLE PRECISION,        -- degrees
    accuracy     DOUBLE PRECISION,        -- meters
    battery      DOUBLE PRECISION,        -- percent, nullable (hardware trackers may not report it)
    recorded_at  TIMESTAMPTZ NOT NULL,    -- device-reported timestamp
    received_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_positions_device_time ON positions(device_id, recorded_at DESC);
CREATE INDEX idx_positions_geom ON positions USING GIST(geom);

ALTER TABLE devices
    ADD CONSTRAINT fk_devices_last_position
    FOREIGN KEY (last_position_id) REFERENCES positions(id) ON DELETE SET NULL;

-- ---------------------------------------------------------------------------
-- Geofences (Step 10)
-- ---------------------------------------------------------------------------

CREATE TABLE geofences (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    geom        GEOGRAPHY(POLYGON, 4326) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_geofences_org_id ON geofences(org_id);
CREATE INDEX idx_geofences_geom ON geofences USING GIST(geom);

CREATE TABLE geofence_events (
    id           BIGSERIAL PRIMARY KEY,
    device_id    UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    geofence_id  UUID NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    type         TEXT NOT NULL CHECK (type IN ('enter', 'exit')),
    occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_geofence_events_device ON geofence_events(device_id, occurred_at DESC);

-- ---------------------------------------------------------------------------
-- Speed violations (Step 16 item 3, Step 18 trigger source)
-- ---------------------------------------------------------------------------

CREATE TABLE speed_events (
    id              BIGSERIAL PRIMARY KEY,
    device_id       UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    speed           DOUBLE PRECISION NOT NULL,
    limit_at_time   DOUBLE PRECISION NOT NULL,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_speed_events_device ON speed_events(device_id, occurred_at DESC);

-- ---------------------------------------------------------------------------
-- Trips (Step 16 item 8, Step 19 per-device thresholds drive segmentation)
-- ---------------------------------------------------------------------------

CREATE TABLE trips (
    id           BIGSERIAL PRIMARY KEY,
    device_id    UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    start_at     TIMESTAMPTZ NOT NULL,
    end_at       TIMESTAMPTZ,
    start_geom   GEOGRAPHY(POINT, 4326) NOT NULL,
    end_geom     GEOGRAPHY(POINT, 4326),
    distance_m   DOUBLE PRECISION
);

CREATE INDEX idx_trips_device ON trips(device_id, start_at DESC);

-- ---------------------------------------------------------------------------
-- Activity logs (Step 16 item 11)
-- ---------------------------------------------------------------------------

CREATE TABLE activity_logs (
    id             BIGSERIAL PRIMARY KEY,
    org_id         UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    actor_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    action         TEXT NOT NULL,
    target_type    TEXT,
    target_id      TEXT,
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_activity_logs_org ON activity_logs(org_id, occurred_at DESC);

-- ---------------------------------------------------------------------------
-- Notification settings (Step 18)
-- ---------------------------------------------------------------------------

CREATE TABLE notification_settings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE, -- nullable: org-wide default if null
    channel     TEXT NOT NULL CHECK (channel IN ('dashboard', 'email', 'web_push')),
    enabled     BOOLEAN NOT NULL DEFAULT true,
    target      TEXT, -- email address, or push subscription JSON serialized as text
    UNIQUE (org_id, user_id, channel)
);

CREATE INDEX idx_notification_settings_org ON notification_settings(org_id);
