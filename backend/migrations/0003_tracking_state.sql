-- Devices can now report whether tracking is intentionally on/off so the
-- dashboard shows "Paused" vs generic "Stationary".
ALTER TABLE devices ADD COLUMN IF NOT EXISTS tracking_active BOOLEAN NOT NULL DEFAULT true;
