-- +goose Up
CREATE TABLE portfolio_asset_snapshots (
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    asset_ref TEXT NOT NULL CHECK (asset_ref = btrim(asset_ref) AND length(asset_ref) > 0),
    name TEXT NOT NULL CHECK (name = btrim(name) AND length(name) > 0),
    asset_class TEXT NOT NULL CHECK (asset_class IN ('cash', 'deposit', 'fixed_income', 'equity', 'fund', 'gold', 'property', 'other')),
    value_minor BIGINT NOT NULL CHECK (value_minor >= 0),
    currency CHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    source_currency CHAR(3) NOT NULL CHECK (source_currency ~ '^[A-Z]{3}$'),
    valuation_as_of TIMESTAMPTZ NOT NULL,
    fx_as_of TIMESTAMPTZ,
    source_account_ref TEXT CHECK (
        source_account_ref IS NULL
        OR (source_account_ref = btrim(source_account_ref) AND length(source_account_ref) > 0)
    ),
    source_kind TEXT NOT NULL CHECK (source_kind IN ('manual', 'import')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (household_id, asset_ref),
    CHECK (source_currency = currency OR fx_as_of IS NOT NULL)
);

-- +goose Down
DROP TABLE portfolio_asset_snapshots;
