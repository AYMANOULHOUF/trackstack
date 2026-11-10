-- 0004_trips_notifications_gateway.sql
-- Automated trip segmentation cursor + hardware device identifiers.

-- Hardware trackers identify themselves by IMEI/serial instead of a bearer
-- token. The protocol gateway forwards that identifier and the API maps it to
-- a device row here.
ALTER TABLE devices ADD COLUMN IF NOT EXISTS external_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_external_id
    ON devices (external_id)
    WHERE external_id IS NOT NULL;

-- Highest positions.id the trip worker has already segmented for this device.
-- Keeping the cursor in the database makes the worker restart-safe.
ALTER TABLE devices ADD COLUMN IF NOT EXISTS trip_cursor_position_id BIGINT NOT NULL DEFAULT 0;

-- The worker looks up the single open trip per device on every pass.
CREATE INDEX IF NOT EXISTS idx_trips_open
    ON trips (device_id)
    WHERE end_at IS NULL;

-- Notification delivery queries enabled channels for one organization.
CREATE INDEX IF NOT EXISTS idx_notification_settings_org_enabled
    ON notification_settings (org_id)
    WHERE enabled;

-- Postgres treats NULLs as distinct in a UNIQUE constraint, so the original
-- UNIQUE (org_id, user_id, channel) never stopped duplicate org-wide rows.
-- Collapse any duplicates, then enforce one row per channel when user_id is
-- NULL so the dashboard's update-then-insert stays idempotent.
DELETE FROM notification_settings a
USING notification_settings b
WHERE a.user_id IS NULL AND b.user_id IS NULL
  AND a.org_id = b.org_id
  AND a.channel = b.channel
  AND a.ctid < b.ctid;

CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_settings_org_channel
    ON notification_settings (org_id, channel)
    WHERE user_id IS NULL;
