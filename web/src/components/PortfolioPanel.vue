<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { deletePortfolioAsset, listPortfolioAssets, upsertPortfolioAsset } from '../api'
import {
  buildPortfolioAssetRequest,
  formFromPortfolioAsset,
  isoToLocalDateTimeInput,
  type PortfolioFormError,
  type PortfolioFormState,
} from '../portfolio-form'
import { formatMoney } from '../money'
import type { PortfolioAssetClass, PortfolioAssetResponse, PortfolioSnapshotSourceKind } from '../types'

const props = defineProps<{
  householdId: number
  defaultCurrency: string
}>()

const items = ref<PortfolioAssetResponse[]>([])
const loading = ref(false)
const saving = ref(false)
const deletingRef = ref('')
const errorCode = ref('')
const formError = ref('')
const formOpen = ref(false)
const editingRef = ref('')
const form = reactive<PortfolioFormState>(newFormState(props.defaultCurrency))

const assetClasses: Array<{ value: PortfolioAssetClass; label: string }> = [
  { value: 'cash', label: '现金' },
  { value: 'deposit', label: '存款' },
  { value: 'fixed_income', label: '固定收益' },
  { value: 'equity', label: '股票 / 权益' },
  { value: 'fund', label: '基金' },
  { value: 'gold', label: '黄金' },
  { value: 'property', label: '房产' },
  { value: 'other', label: '其他' },
]

const sourceKinds: Array<{ value: PortfolioSnapshotSourceKind; label: string }> = [
  { value: 'manual', label: '手工录入' },
  { value: 'import', label: '导入' },
]

const requiresFX = computed(() => {
  const reporting = form.currency.trim().toUpperCase()
  const source = form.sourceCurrency.trim().toUpperCase()
  return reporting.length === 3 && source.length === 3 && reporting !== source
})

async function reload() {
  if (!Number.isSafeInteger(props.householdId) || props.householdId <= 0) {
    items.value = []
    errorCode.value = 'invalid_request'
    return
  }

  loading.value = true
  errorCode.value = ''
  try {
    const response = await listPortfolioAssets(props.householdId)
    items.value = response.items
  } catch (error) {
    errorCode.value = error instanceof Error ? error.message : 'request_failed'
  } finally {
    loading.value = false
  }
}

function openNew() {
  Object.assign(form, newFormState(props.defaultCurrency))
  editingRef.value = ''
  formError.value = ''
  formOpen.value = true
}

function openEdit(asset: PortfolioAssetResponse) {
  Object.assign(form, formFromPortfolioAsset(asset))
  editingRef.value = asset.asset_ref
  formError.value = ''
  formOpen.value = true
}

function closeForm() {
  if (saving.value) return
  formOpen.value = false
  editingRef.value = ''
  formError.value = ''
}

async function submit() {
  const result = buildPortfolioAssetRequest(form)
  if (!result.ok) {
    formError.value = formErrorMessage(result.error)
    return
  }
  if (!Number.isSafeInteger(props.householdId) || props.householdId <= 0) {
    formError.value = '家庭 ID 无效。'
    return
  }

  saving.value = true
  formError.value = ''
  try {
    await upsertPortfolioAsset(props.householdId, result.assetRef, result.request)
    formOpen.value = false
    editingRef.value = ''
    await reload()
  } catch (error) {
    formError.value = `保存失败：${error instanceof Error ? error.message : 'request_failed'}`
  } finally {
    saving.value = false
  }
}

async function remove(asset: PortfolioAssetResponse) {
  if (!window.confirm(`确认删除资产快照「${asset.name}」？此操作不会删除账本交易。`)) return

  deletingRef.value = asset.asset_ref
  errorCode.value = ''
  try {
    await deletePortfolioAsset(props.householdId, asset.asset_ref)
    if (editingRef.value === asset.asset_ref) closeForm()
    await reload()
  } catch (error) {
    errorCode.value = error instanceof Error ? error.message : 'request_failed'
  } finally {
    deletingRef.value = ''
  }
}

function newFormState(currency: string): PortfolioFormState {
  const normalizedCurrency = /^[A-Za-z]{3}$/.test(currency.trim()) ? currency.trim().toUpperCase() : 'CNY'
  return {
    assetRef: '',
    name: '',
    assetClass: 'other',
    amount: '',
    currency: normalizedCurrency,
    sourceCurrency: normalizedCurrency,
    valuationAsOf: isoToLocalDateTimeInput(new Date().toISOString()),
    fxAsOf: '',
    sourceAccountRef: '',
    sourceKind: 'manual',
  }
}

function classLabel(value: PortfolioAssetClass): string {
  return assetClasses.find((item) => item.value === value)?.label ?? value
}

function sourceKindLabel(value: PortfolioSnapshotSourceKind): string {
  return sourceKinds.find((item) => item.value === value)?.label ?? value
}

function formattedValuationTime(value: string): string {
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString('zh-CN')
}

function formErrorMessage(error: PortfolioFormError): string {
  const messages: Record<PortfolioFormError, string> = {
    asset_ref_required: '请输入资产引用 ID。',
    name_required: '请输入资产名称。',
    invalid_asset_class: '请选择有效的资产类别。',
    invalid_amount: '金额格式无效，请输入非负十进制金额。',
    amount_out_of_range: '金额超出 Finance Core 支持范围。',
    invalid_currency: '报告币种必须是 3 位货币代码。',
    invalid_source_currency: '来源币种必须是 3 位货币代码。',
    invalid_valuation_as_of: '估值时间无效。',
    fx_as_of_required: '来源币种与报告币种不同时必须填写 FX 时间。',
    invalid_fx_as_of: 'FX 时间无效。',
    invalid_source_kind: '来源类型无效。',
  }
  return messages[error]
}

watch(
  () => props.householdId,
  () => {
    formOpen.value = false
    editingRef.value = ''
    void reload()
  },
)

watch(
  () => props.defaultCurrency,
  (currency) => {
    if (formOpen.value) return
    Object.assign(form, newFormState(currency))
  },
)

onMounted(() => {
  void reload()
})
</script>

<template>
  <section class="panel portfolio-panel" aria-labelledby="portfolio-heading">
    <div class="panel-heading portfolio-heading">
      <div>
        <p class="eyebrow">PORTFOLIO</p>
        <h2 id="portfolio-heading">资产快照</h2>
        <p class="portfolio-copy">录入当前资产事实；Finance Core 负责确定性分类汇总，不在浏览器计算投资组合。</p>
      </div>
      <button type="button" class="portfolio-add" :disabled="loading || saving" @click="openNew">新增资产</button>
    </div>

    <div v-if="errorCode" class="portfolio-error" role="alert">
      资产快照加载失败：{{ errorCode }}
      <button type="button" class="button-link" @click="reload">重试</button>
    </div>

    <form v-if="formOpen" class="portfolio-form" @submit.prevent="submit">
      <div class="portfolio-form__heading">
        <strong>{{ editingRef ? '编辑资产快照' : '新增资产快照' }}</strong>
        <button type="button" class="button-secondary" :disabled="saving" @click="closeForm">取消</button>
      </div>

      <div class="portfolio-form__grid">
        <label>
          <span>资产引用 ID</span>
          <input v-model="form.assetRef" :disabled="Boolean(editingRef)" autocomplete="off" placeholder="例如 property:home" required />
        </label>
        <label>
          <span>资产名称</span>
          <input v-model="form.name" autocomplete="off" placeholder="例如 自住房" required />
        </label>
        <label>
          <span>资产类别</span>
          <select v-model="form.assetClass">
            <option v-for="item in assetClasses" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>
        <label>
          <span>当前估值</span>
          <input v-model="form.amount" inputmode="decimal" autocomplete="off" placeholder="0.00" required />
        </label>
        <label>
          <span>报告币种</span>
          <input v-model="form.currency" maxlength="3" autocomplete="off" placeholder="CNY" required />
        </label>
        <label>
          <span>来源币种</span>
          <input v-model="form.sourceCurrency" maxlength="3" autocomplete="off" placeholder="CNY" required />
        </label>
        <label>
          <span>估值时间</span>
          <input v-model="form.valuationAsOf" type="datetime-local" required />
        </label>
        <label v-if="requiresFX">
          <span>FX 时间</span>
          <input v-model="form.fxAsOf" type="datetime-local" required />
        </label>
        <label>
          <span>关联账本账户（可选）</span>
          <input v-model="form.sourceAccountRef" autocomplete="off" placeholder="例如 broker-1" />
        </label>
        <label>
          <span>来源</span>
          <select v-model="form.sourceKind">
            <option v-for="item in sourceKinds" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>
      </div>

      <p v-if="requiresFX" class="portfolio-hint">当前值必须已经换算为报告币种；这里仅记录该换算所使用的 FX 时间，不自动查询汇率。</p>
      <p v-if="formError" class="portfolio-form__error" role="alert">{{ formError }}</p>
      <div class="portfolio-form__actions">
        <button type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存快照' }}</button>
      </div>
    </form>

    <div v-if="loading && !items.length" class="portfolio-empty" aria-live="polite">正在读取资产快照…</div>
    <div v-else-if="items.length" class="portfolio-list">
      <article v-for="asset in items" :key="asset.asset_ref" class="portfolio-row">
        <div class="portfolio-row__identity">
          <div class="portfolio-row__title">
            <strong>{{ asset.name }}</strong>
            <span class="portfolio-chip">{{ classLabel(asset.asset_class) }}</span>
          </div>
          <span class="portfolio-meta">{{ asset.asset_ref }} · {{ sourceKindLabel(asset.source_kind) }}</span>
          <span class="portfolio-meta">
            估值 {{ formattedValuationTime(asset.valuation_as_of) }}
            <template v-if="asset.source_account_ref"> · 账户 {{ asset.source_account_ref }}</template>
          </span>
          <span v-if="asset.source_currency !== asset.currency" class="portfolio-meta">
            来源币种 {{ asset.source_currency }} · 已记录 FX 时间 {{ asset.fx_as_of ? formattedValuationTime(asset.fx_as_of) : '—' }}
          </span>
        </div>
        <div class="portfolio-row__side">
          <strong class="portfolio-value">{{ formatMoney({ minor: asset.value_minor, currency: asset.currency }) }}</strong>
          <div class="portfolio-actions">
            <button type="button" class="button-secondary" :disabled="saving || Boolean(deletingRef)" @click="openEdit(asset)">编辑</button>
            <button type="button" class="button-danger" :disabled="saving || Boolean(deletingRef)" @click="remove(asset)">
              {{ deletingRef === asset.asset_ref ? '删除中…' : '删除' }}
            </button>
          </div>
        </div>
      </article>
    </div>
    <p v-else-if="!loading" class="portfolio-empty">还没有显式资产快照。新增后，Finance Core 会把它们纳入确定性资产分类汇总。</p>
  </section>
</template>

<style scoped>
.portfolio-panel {
  margin-top: 0.9rem;
}

.portfolio-heading {
  align-items: center;
}

.portfolio-copy {
  max-width: 680px;
  margin: 0.4rem 0 0;
  color: var(--muted);
  font-size: 0.8rem;
  line-height: 1.55;
}

.portfolio-add {
  flex: 0 0 auto;
}

.portfolio-error,
.portfolio-form__error {
  margin: 0 0 0.8rem;
  border-radius: 10px;
  padding: 0.7rem 0.8rem;
  color: var(--danger);
  background: #fff6f4;
  font-size: 0.82rem;
}

.button-link {
  margin-left: 0.35rem;
  border: 0;
  background: transparent;
  color: var(--accent);
  padding: 0;
}

.portfolio-form {
  margin-bottom: 0.95rem;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface-muted);
  padding: 0.9rem;
}

.portfolio-form__heading,
.portfolio-form__actions,
.portfolio-row__title,
.portfolio-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.portfolio-form__heading {
  justify-content: space-between;
  margin-bottom: 0.8rem;
}

.portfolio-form__grid {
  display: grid;
  gap: 0.7rem;
}

.portfolio-form label {
  display: grid;
  gap: 0.32rem;
  color: var(--muted);
  font-size: 0.78rem;
  font-weight: 700;
}

.portfolio-form input,
.portfolio-form select {
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface);
  color: var(--text);
  padding: 0.68rem 0.75rem;
  font: inherit;
  outline: none;
}

.portfolio-form input:focus,
.portfolio-form select:focus {
  outline: 3px solid rgb(37 99 235 / 20%);
  outline-offset: 2px;
}

.portfolio-hint {
  margin: 0.75rem 0 0;
  color: var(--muted);
  font-size: 0.78rem;
  line-height: 1.55;
}

.portfolio-form__error {
  margin-top: 0.75rem;
}

.portfolio-form__actions {
  justify-content: flex-end;
  margin-top: 0.8rem;
}

.portfolio-list {
  display: grid;
}

.portfolio-row {
  display: grid;
  gap: 0.7rem;
  border-top: 1px solid var(--border);
  padding: 0.9rem 0;
}

.portfolio-row:first-child {
  border-top: 0;
  padding-top: 0.1rem;
}

.portfolio-row__identity,
.portfolio-row__side {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.portfolio-row__title {
  flex-wrap: wrap;
}

.portfolio-chip {
  border-radius: 999px;
  background: #eff6ff;
  color: var(--accent-strong);
  padding: 0.18rem 0.5rem;
  font-size: 0.68rem;
  font-weight: 800;
}

.portfolio-meta,
.portfolio-empty {
  color: var(--muted);
  font-size: 0.78rem;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.portfolio-empty {
  margin: 0;
  padding: 0.85rem 0;
}

.portfolio-value {
  font-size: 1.05rem;
  overflow-wrap: anywhere;
}

.portfolio-actions {
  flex-wrap: wrap;
}

.button-secondary,
.button-danger {
  padding: 0.5rem 0.72rem;
  font-size: 0.76rem;
}

.button-secondary {
  border-color: var(--border);
  background: var(--surface);
  color: var(--text);
}

.button-secondary:hover:not(:disabled) {
  background: #eef2f7;
}

.button-danger {
  border-color: #fecaca;
  background: #fff7f7;
  color: var(--danger);
}

.button-danger:hover:not(:disabled) {
  background: #fee2e2;
}

@media (min-width: 720px) {
  .portfolio-form__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .portfolio-row {
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
  }

  .portfolio-row__side {
    justify-items: end;
  }
}
</style>
