import { moneyInputFromMinor, parseMoneyInput } from './money'
import type {
  PortfolioAssetClass,
  PortfolioAssetResponse,
  PortfolioAssetUpsertRequest,
  PortfolioSnapshotSourceKind,
} from './types'

const maxInt64Minor = 9_223_372_036_854_775_807n
const assetClasses = new Set<PortfolioAssetClass>([
  'cash',
  'deposit',
  'fixed_income',
  'equity',
  'fund',
  'gold',
  'property',
  'other',
])
const sourceKinds = new Set<PortfolioSnapshotSourceKind>(['manual', 'import'])

export interface PortfolioFormState {
  assetRef: string
  name: string
  assetClass: PortfolioAssetClass
  amount: string
  currency: string
  sourceCurrency: string
  valuationAsOf: string
  fxAsOf: string
  sourceAccountRef: string
  sourceKind: PortfolioSnapshotSourceKind
}

export type PortfolioFormError =
  | 'asset_ref_required'
  | 'name_required'
  | 'invalid_asset_class'
  | 'invalid_amount'
  | 'amount_out_of_range'
  | 'invalid_currency'
  | 'invalid_source_currency'
  | 'invalid_valuation_as_of'
  | 'fx_as_of_required'
  | 'invalid_fx_as_of'
  | 'invalid_source_kind'

export type PortfolioFormBuildResult =
  | { ok: true; assetRef: string; request: PortfolioAssetUpsertRequest }
  | { ok: false; error: PortfolioFormError }

export function buildPortfolioAssetRequest(form: PortfolioFormState): PortfolioFormBuildResult {
  const assetRef = form.assetRef.trim()
  if (!assetRef) return { ok: false, error: 'asset_ref_required' }

  const name = form.name.trim()
  if (!name) return { ok: false, error: 'name_required' }

  if (!assetClasses.has(form.assetClass)) return { ok: false, error: 'invalid_asset_class' }

  const currency = form.currency.trim().toUpperCase()
  if (!/^[A-Z]{3}$/.test(currency)) return { ok: false, error: 'invalid_currency' }

  const sourceCurrency = form.sourceCurrency.trim().toUpperCase()
  if (!/^[A-Z]{3}$/.test(sourceCurrency)) return { ok: false, error: 'invalid_source_currency' }

  const valueMinor = parseMoneyInput(form.amount, currency)
  if (valueMinor === null) return { ok: false, error: 'invalid_amount' }
  if (BigInt(valueMinor) > maxInt64Minor) return { ok: false, error: 'amount_out_of_range' }

  const valuationAsOf = localDateTimeInputToISO(form.valuationAsOf)
  if (!valuationAsOf) return { ok: false, error: 'invalid_valuation_as_of' }

  if (!sourceKinds.has(form.sourceKind)) return { ok: false, error: 'invalid_source_kind' }

  const request: PortfolioAssetUpsertRequest = {
    name,
    asset_class: form.assetClass,
    value_minor: valueMinor,
    currency,
    source_currency: sourceCurrency,
    valuation_as_of: valuationAsOf,
    source_kind: form.sourceKind,
  }

  const sourceAccountRef = form.sourceAccountRef.trim()
  if (sourceAccountRef) request.source_account_ref = sourceAccountRef

  if (sourceCurrency !== currency) {
    if (!form.fxAsOf.trim()) return { ok: false, error: 'fx_as_of_required' }
    const fxAsOf = localDateTimeInputToISO(form.fxAsOf)
    if (!fxAsOf) return { ok: false, error: 'invalid_fx_as_of' }
    request.fx_as_of = fxAsOf
  }

  return { ok: true, assetRef, request }
}

export function formFromPortfolioAsset(asset: PortfolioAssetResponse): PortfolioFormState {
  return {
    assetRef: asset.asset_ref,
    name: asset.name,
    assetClass: asset.asset_class,
    amount: moneyInputFromMinor(asset.value_minor, asset.currency),
    currency: asset.currency,
    sourceCurrency: asset.source_currency,
    valuationAsOf: isoToLocalDateTimeInput(asset.valuation_as_of),
    fxAsOf: asset.fx_as_of ? isoToLocalDateTimeInput(asset.fx_as_of) : '',
    sourceAccountRef: asset.source_account_ref ?? '',
    sourceKind: asset.source_kind,
  }
}

export function localDateTimeInputToISO(value: string): string | null {
  const normalized = value.trim()
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(normalized)) return null

  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) return null
  const iso = date.toISOString()
  if (isoToLocalDateTimeInput(iso) !== normalized) return null
  return iso
}

export function isoToLocalDateTimeInput(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''

  return [
    date.getFullYear().toString().padStart(4, '0'),
    '-',
    pad2(date.getMonth() + 1),
    '-',
    pad2(date.getDate()),
    'T',
    pad2(date.getHours()),
    ':',
    pad2(date.getMinutes()),
  ].join('')
}

function pad2(value: number): string {
  return String(value).padStart(2, '0')
}
