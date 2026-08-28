-- +goose Up
ALTER TABLE auth_users
    ADD COLUMN role TEXT NOT NULL DEFAULT 'owner'
    CHECK (role IN ('owner', 'editor', 'viewer'));

CREATE INDEX auth_users_household_role_active_idx
    ON auth_users(household_id, role)
    WHERE disabled_at IS NULL;

-- +goose Down
DROP INDEX auth_users_household_role_active_idx;
ALTER TABLE auth_users DROP COLUMN role;
