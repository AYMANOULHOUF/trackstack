-- 0002_admin_multiorg.sql
-- Single global admin + many-to-many device/org + unassigned (pending) devices.

-- The global admin is a user with no org and is_admin = true, seeded from
-- ADMIN_EMAIL / ADMIN_PASSWORD env at api-service startup.
ALTER TABLE users ALTER COLUMN org_id DROP NOT NULL;
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;

-- devices.org_id is no longer the source of truth for scoping; membership is
-- now via device_orgs. Freshly enrolled phones have NULL org_id and no
-- device_orgs rows => "unassigned", visible only to the admin.
ALTER TABLE devices ALTER COLUMN org_id DROP NOT NULL;

CREATE TABLE device_orgs (
    device_id  UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (device_id, org_id)
);
CREATE INDEX idx_device_orgs_org ON device_orgs(org_id);
CREATE INDEX idx_device_orgs_device ON device_orgs(device_id);
