-- name: ListPortfolioAssetSnapshotsByHousehold :many
SELECT
    household_id,
    asset_ref,
    name,
    asset_class,
    value_minor,
    currency,
    source_currency,
    valuation_as_of,
    fx_as_of,
    source_account_ref,
    source_kind
FROM portfolio_asset_snapshots
WHERE household_id = $1
ORDER BY asset_ref ASC;

-- name: UpsertPortfolioAssetSnapshot :one
INSERT INTO portfolio_asset_snapshots (
    household_id,
    asset_ref,
    name,
    asset_class,
    value_minor,
    currency,
    source_currency,
    valuation_as_of,
    fx_as_of,
    source_account_ref,
    source_kind
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (household_id, asset_ref) DO UPDATE SET
    name = EXCLUDED.name,
    asset_class = EXCLUDED.asset_class,
    value_minor = EXCLUDED.value_minor,
    currency = EXCLUDED.currency,
    source_currency = EXCLUDED.source_currency,
    valuation_as_of = EXCLUDED.valuation_as_of,
    fx_as_of = EXCLUDED.fx_as_of,
    source_account_ref = EXCLUDED.source_account_ref,
    source_kind = EXCLUDED.source_kind,
    updated_at = CURRENT_TIMESTAMP
RETURNING
    household_id,
    asset_ref,
    name,
    asset_class,
    value_minor,
    currency,
    source_currency,
    valuation_as_of,
    fx_as_of,
    source_account_ref,
    source_kind;

-- name: DeletePortfolioAssetSnapshot :exec
DELETE FROM portfolio_asset_snapshots
WHERE household_id = $1
  AND asset_ref = $2;
