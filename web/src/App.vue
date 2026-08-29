<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref, watch } from 'vue'
import { askAdvisor, loadDashboard } from './api'
import {
  bootstrapSession,
  clearAuthState,
  currentAuthEpoch,
  currentAuthState,
  logout,
  subscribeAuthState,
  type AuthState,
} from './auth'
import ConfirmDialog from './components/ConfirmDialog.vue'
import HouseholdMembersPanel from './components/HouseholdMembersPanel.vue'
import LoginPanel from './components/LoginPanel.vue'
import MetricCard from './components/MetricCard.vue'
import PortfolioPanel from './components/PortfolioPanel.vue'
import TOTPPanel from './components/TOTPPanel.vue'
import { errorText } from './errors'
import { chartValue, formatMoney, formatPercent } from './money'
import type { AdvisorResponse, DashboardData, HouseholdRole } from './types'

// ECharts 体积大且仅登录后需要,异步拆包减小首屏 bundle
const CashflowChart = defineAsyncComponent(() => import('./components/CashflowChart.vue'))

const authState = ref<AuthState>(currentAuthState())

// 近 24 个月的 YYYY-MM 选项,type="month" 在 iOS Safari 不可用
const MONTH_OPTIONS_COUNT = 24

// URL ?m=YYYY-MM 恢复上次查看的月份:格式合法且落在可选区间内才生效,否则回落到当前月
function initialPeriod(): string {
  const candidate = new URLSearchParams(window.location.search).get('m')
  if (candidate && /^\d{4}-\d{2}$/.test(candidate)) {
    const [year, month] = candidate.split('-').map(Number)
    if (year && month) {
      const now = new Date()
      const offset = (now.getFullYear() - year) * 12 + (now.getMonth() + 1 - month)
      if (offset >= 0 && offset < MONTH_OPTIONS_COUNT) return candidate
    }
  }
  return currentPeriod()
}

const period = ref(initialPeriod())
const data = ref<DashboardData | null>(null)
// dataPeriod 记录 data 实际对应的月份:切换月份后、新数据到达前,二者不同即"数据待刷新"
const dataPeriod = ref('')
const loading = ref(false)
const errorCode = ref('')
const advisorQuestion = ref('')
const advisorResult = ref<AdvisorResponse | null>(null)
// 提问时用户正在查看的月份:AI 结果锚定该上下文,切月后不产生"结果属于哪个月"的歧义
const advisorPeriod = ref('')
const advisorErrorCode = ref('')
const advisorLoading = ref(false)
const signingOut = ref(false)

const unsubscribeAuth = subscribeAuthState((next) => {
  authState.value = next
})
onUnmounted(unsubscribeAuth)

const needsSecondFactor = computed(
  () => authState.value.phase === 'enroll_totp' || authState.value.phase === 'verify_totp',
)
const needsRecoveryCodeAcknowledgement = computed(
  () => authState.value.authenticated && authState.value.recoveryCodes.length > 0,
)
const dashboardReady = computed(
  () => authState.value.authenticated && !needsRecoveryCodeAcknowledgement.value,
)
const warnings = computed(() => {
  if (!data.value) return []
  return [
    ...(data.value.overview.warnings ?? []),
    ...(data.value.cashflow.warnings ?? []),
    ...(data.value.budget.warnings ?? []),
    ...(data.value.debts.warnings ?? []),
    ...(data.value.goals.warnings ?? []),
  ].filter((warning, index, all) => all.indexOf(warning) === index)
})

// 请求序号守卫:快速连续切月时,只有最后一次请求的结果允许落盘
let refreshSeq = 0

async function refresh() {
  if (!dashboardReady.value || !/^\d{4}-\d{2}$/.test(period.value)) {
    if (dashboardReady.value) errorCode.value = 'invalid_request'
    return
  }

  const requestedPeriod = period.value
  const seq = ++refreshSeq
  const authEpoch = currentAuthEpoch()
  loading.value = true
  errorCode.value = ''
  try {
    const next = await loadDashboard(requestedPeriod)
    if (seq === refreshSeq && authEpoch === currentAuthEpoch() && dashboardReady.value) {
      data.value = next
      dataPeriod.value = requestedPeriod
    }
  } catch (error) {
    if (seq === refreshSeq && authEpoch === currentAuthEpoch()) {
      errorCode.value = error instanceof Error ? error.message : 'request_failed'
    }
  } finally {
    if (seq === refreshSeq && authEpoch === currentAuthEpoch() && period.value === requestedPeriod) {
      loading.value = false
    }
  }
}

async function submitAdvisor() {
  const question = advisorQuestion.value.trim()
  if (!dashboardReady.value || !question) return

  const authEpoch = currentAuthEpoch()
  advisorLoading.value = true
  advisorResult.value = null
  // 请求失败(网络/5xx)与服务端真 blocked 是两回事:失败不伪装成安全拦截,也不覆盖上次成功回答
  advisorErrorCode.value = ''
  advisorPeriod.value = dataPeriod.value || period.value
  try {
    const next = await askAdvisor(question, true)
    if (authEpoch === currentAuthEpoch() && dashboardReady.value) advisorResult.value = next
  } catch (error) {
    if (authEpoch === currentAuthEpoch()) {
      advisorErrorCode.value = error instanceof Error ? error.message : 'request_failed'
    }
  } finally {
    if (authEpoch === currentAuthEpoch()) advisorLoading.value = false
  }
}

async function signOut() {
  if (signingOut.value) return
  signingOut.value = true
  errorCode.value = ''
  try {
    await logout()
    data.value = null
    dataPeriod.value = ''
    advisorResult.value = null
    advisorPeriod.value = ''
    advisorErrorCode.value = ''
    loading.value = false
    advisorLoading.value = false
    advisorQuestion.value = ''
  } catch (error) {
    errorCode.value = error instanceof Error ? error.message : 'logout_failed'
  } finally {
    signingOut.value = false
  }
}

function currentPeriod(): string {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

function periodLabel(value: string): string {
  const [year, month] = value.split('-')
  if (!year || !month) return value
  return `${year} 年 ${Number(month)} 月`
}

// 当前展示的 data 是否属于历史月(非当前自然月)
const isHistoricalView = computed(
  () => Boolean(data.value) && Boolean(dataPeriod.value) && dataPeriod.value !== currentPeriod(),
)
const viewedPeriodLabel = computed(() => periodLabel(dataPeriod.value || period.value))
const viewingPeriodLabel = computed(() => periodLabel(period.value))
// 历史月时面板标题如实命名,不再错标"本月"
const cashflowTitle = computed(() => (isHistoricalView.value ? `${viewedPeriodLabel.value}现金流` : '本月现金流'))

// 月份变更即自动加载,无需再点刷新;同步进 URL 供刷新/分享后回到同一月
watch(period, (value) => {
  syncPeriodToUrl(value)
  if (dashboardReady.value) void refresh()
})

// 用 replaceState 而非 push:切月不产生历史记录
function syncPeriodToUrl(value: string): void {
  const url = new URL(window.location.href)
  url.searchParams.set('m', value)
  window.history.replaceState(window.history.state, '', url)
}

// 月份步进:在可选区间内平移一个月
function stepPeriod(delta: number): void {
  const [year, month] = period.value.split('-').map(Number)
  if (!year || !month) return
  const d = new Date(year, month - 1 + delta, 1)
  const next = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  if (periodOptions.value.some((o) => o.value === next)) period.value = next
}

// 键盘快捷键:←/→ 切月(优先作用于月份控件;聚焦输入框时不拦截),/ 聚焦 AI 顾问
function onKeydown(event: KeyboardEvent): void {
  const tag = document.activeElement?.tagName
  const inField = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
  if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return

  if (event.key === 'ArrowLeft' && !inField) {
    event.preventDefault()
    stepPeriod(-1)
  } else if (event.key === 'ArrowRight' && !inField) {
    event.preventDefault()
    stepPeriod(1)
  } else if (event.key === '/' && !inField) {
    event.preventDefault()
    advisorInput.value?.focus()
  }
}

const advisorInput = ref<HTMLTextAreaElement | null>(null)
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

// Ctrl/Cmd+Enter 直接提交提问;请求进行中忽略,与按钮 disabled 语义保持一致
function onAdvisorKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter' && !advisorLoading.value) {
    event.preventDefault()
    void submitAdvisor()
  }
}

const periodOptions = computed(() => {
  const now = new Date()
  const options: Array<{ value: string; label: string }> = []
  for (let offset = 0; offset < MONTH_OPTIONS_COUNT; offset += 1) {
    const d = new Date(now.getFullYear(), now.getMonth() - offset, 1)
    const value = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    options.push({ value, label: `${d.getFullYear()} 年 ${d.getMonth() + 1} 月` })
  }
  return options
})

// 负面语义判定:金额 minor 为负即 danger 态
function isNegative(value: { currency: string; minor: string }): boolean {
  try {
    return BigInt(value.minor) < 0n
  } catch {
    return false
  }
}

// 预算使用率分级:utilization 是 ratio(0.75=75%),≥1.0 超支(danger),0.8-0.99 预警(amber)
function budgetUtilizationTone(line: { utilization?: string }): 'danger' | 'warn' | null {
  if (!line.utilization) return null
  const normalized = line.utilization.trim()
  if (!/^\d+(\.\d+)?$/.test(normalized)) return null
  const ratio = Number(normalized)
  if (!Number.isFinite(ratio)) return null
  if (ratio >= 1) return 'danger'
  if (ratio >= 0.8) return 'warn'
  return null
}

// 模板内每行只调两次分级(类名 + 文字后缀),避免同一行重复计算三次
function budgetToneClass(line: { utilization?: string }): string | null {
  const tone = budgetUtilizationTone(line)
  if (tone === 'danger') return 'list-row--danger'
  if (tone === 'warn') return 'list-row--warn'
  return null
}

function budgetToneSuffix(line: { utilization?: string }): string {
  const tone = budgetUtilizationTone(line)
  if (tone === 'danger') return ' · 已超支'
  if (tone === 'warn') return ' · 接近上限'
  return ''
}

function qualityLabel(value: string): string {
  const labels: Record<string, string> = {
    good: '数据完整',
    partial: '数据部分缺失',
    stale: '数据已过期',
    unknown: '数据状态未知',
  }
  return labels[value] ?? value
}

function goalStatus(value: string): string {
  const labels: Record<string, string> = {
    completed: '已完成',
    on_track: '按计划',
    behind: '进度落后',
    conflicting: '资金冲突',
    infeasible: '当前不可行',
  }
  return labels[value] ?? value
}

function debtTypeLabel(value?: string): string {
  const labels: Record<string, string> = {
    mortgage: '房贷',
    credit_card: '信用卡',
    consumer_loan: '消费贷',
    installment: '分期',
    other: '其他债务',
  }
  if (!value) return '—'
  return labels[value] ?? value
}

function repaymentLabel(value?: string): string {
  const labels: Record<string, string> = {
    annuity: '等额本息',
    equal_principal: '等额本金',
    revolving: '循环额度',
    custom: '自定义还款',
  }
  if (!value) return '—'
  return labels[value] ?? value
}

function budgetKindLabel(value?: string): string {
  const labels: Record<string, string> = {
    essential: '必要支出',
    flexible: '弹性支出',
    goal: '目标储蓄',
    saving: '储蓄',
    investment: '投资',
    debt: '还债',
  }
  if (!value) return ''
  return labels[value] ?? value
}

function roleLabel(role: HouseholdRole | null): string {
  // 与成员面板的 roles 词汇保持一致,顶栏和成员列表不出现两套说法
  const labels: Record<HouseholdRole, string> = {
    owner: 'Owner · 管理员',
    editor: 'Editor · 可编辑',
    viewer: 'Viewer · 只读',
  }
  return role ? labels[role] : '—'
}

watch(
  () => [authState.value.authenticated, authState.value.recoveryCodes.length] as const,
  ([authenticated, recoveryCodeCount]) => {
    if (authenticated && recoveryCodeCount === 0) {
      void refresh()
      return
    }
    data.value = null
    dataPeriod.value = ''
    advisorResult.value = null
    advisorPeriod.value = ''
    advisorErrorCode.value = ''
  },
)

onMounted(async () => {
  // 挂载时把初始月份写回 URL(直接访问无 ?m= 时补齐为当前月)
  syncPeriodToUrl(period.value)
  try {
    await bootstrapSession()
  } catch {
    clearAuthState()
  }
})
</script>

<template>
  <main v-if="authState.phase === 'checking'" class="session-check" aria-live="polite">
    <div>
      <p class="eyebrow">FAMILY FINANCE OS</p>
      <strong>正在验证会话…</strong>
    </div>
  </main>

  <LoginPanel v-else-if="authState.phase === 'login'" />

  <TOTPPanel
    v-else-if="needsSecondFactor || needsRecoveryCodeAcknowledgement"
    :state="authState"
  />

  <div v-else-if="dashboardReady" class="app-shell">
    <header class="topbar">
      <div>
        <p class="eyebrow">FAMILY FINANCE OS</p>
        <h1>家庭财务总览</h1>
        <p class="subtle">确定性计算为准，AI 只负责解释与规划。</p>
      </div>
      <div class="topbar-actions">
        <span class="signed-in-user">{{ authState.username || 'Finance 用户' }} · {{ roleLabel(authState.role) }}</span>
        <form class="context-controls" @submit.prevent="refresh">
          <label>
            <span>月份</span>
            <div class="period-stepper">
              <button type="button" class="period-step" aria-label="上一月" @click="stepPeriod(-1)">‹</button>
              <select v-model="period" aria-label="月份">
                <option v-for="option in periodOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
              <button type="button" class="period-step" aria-label="下一月" @click="stepPeriod(1)">›</button>
            </div>
            <!-- 步进期间允许排队:refreshSeq 守卫会丢弃过期响应,只在非当前月显示 -->
            <button v-if="period !== currentPeriod()" type="button" class="button-link today-button" aria-label="回到本月" @click="period = currentPeriod()">本月</button>
          </label>
          <button type="submit" :disabled="loading">{{ loading ? '读取中…' : '刷新' }}</button>
        </form>
        <button type="button" class="button-secondary" :disabled="signingOut" @click="signOut">
          {{ signingOut ? '退出中…' : '退出登录' }}
        </button>
        <span class="keyboard-hint" title="在月份控件或输入框外按 ←/→ 切换月份，按 / 聚焦 AI 顾问">←/→ 切月 · / 顾问</span>
      </div>
    </header>

    <main>
      <div v-if="errorCode" class="notice notice--error" role="alert">
        操作失败：{{ errorText(errorCode) }}。{{ data && dataPeriod ? `以下仍为 ${viewedPeriodLabel} 的数据，可能已不是最新。请检查 Finance Core、账本同步或网络状态后重试。` : '请检查 Finance Core、账本同步或网络状态。' }}
      </div>

      <template v-if="data">
        <section class="status-row" aria-label="数据状态" aria-live="polite">
          <span v-if="isHistoricalView" class="status-pill status-pill--period">{{ viewedPeriodLabel }}（历史月）</span>
          <span class="status-pill" :data-quality="data.overview.quality">{{ qualityLabel(data.overview.quality) }}</span>
          <span v-if="loading">正在读取 {{ viewingPeriodLabel }} 数据…</span>
          <template v-else>
            <span>数据截至 {{ new Date(data.overview.data_as_of).toLocaleString('zh-CN') }}</span>
            <span v-if="dataPeriod && dataPeriod !== period && !errorCode">正在切换到 {{ viewingPeriodLabel }}…</span>
          </template>
        </section>

        <section class="metric-grid" aria-label="核心指标">
          <MetricCard label="安全可消费" :value="formatMoney(data.overview.safe_to_spend)" hint="本月扣除必要承诺与应急底线后可自由支配的金额" emphasis :danger="isNegative(data.overview.safe_to_spend)" danger-label="为负" />
          <MetricCard label="家庭净资产" :value="formatMoney(data.overview.net_worth)" :danger="isNegative(data.overview.net_worth)" danger-label="为负" />
          <MetricCard label="本月净现金流" :value="formatMoney(data.overview.net_cashflow)" :hint="`储蓄率 ${formatPercent(data.overview.savings_rate)}`" :danger="isNegative(data.overview.net_cashflow)" danger-label="为负" />
          <MetricCard label="应急资金覆盖" :value="data.overview.emergency_months ? `${data.overview.emergency_months} 个月` : '—'" :hint="`总债务 ${formatMoney(data.overview.total_debt)}`" />
        </section>

        <section v-if="warnings.length" class="notice" aria-label="数据质量提醒">
          <strong>需要关注的数据质量</strong>
          <ul>
            <li v-for="warning in warnings" :key="warning">{{ warning }}</li>
          </ul>
        </section>

        <div class="content-grid">
          <section class="panel panel--chart">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">CASHFLOW</p>
                <h2>{{ cashflowTitle }}</h2>
              </div>
              <span class="subtle">{{ viewedPeriodLabel }}</span>
            </div>
            <CashflowChart
              :income="chartValue(data.cashflow.income)"
              :expense="chartValue(data.cashflow.expense)"
              :income-label="formatMoney(data.cashflow.income)"
              :expense-label="formatMoney(data.cashflow.expense)"
            />
            <div class="split-summary">
              <div><span>收入</span><strong>{{ formatMoney(data.cashflow.income) }}</strong></div>
              <div><span>支出</span><strong>{{ formatMoney(data.cashflow.expense) }}</strong></div>
            </div>
          </section>

          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">BUDGET</p>
                <h2>预算执行</h2>
              </div>
            </div>
            <div v-if="data.budget.lines.length" class="stack-list">
              <article v-for="line in data.budget.lines" :key="`${line.kind}:${line.external_category_ref ?? line.semantic_group}`" class="list-row" :class="budgetToneClass(line)">
                <div>
                  <strong>{{ line.semantic_group || line.external_category_ref || budgetKindLabel(line.kind) || line.kind }}</strong>
                  <span>{{ budgetKindLabel(line.kind) || line.kind }} · 使用 {{ formatPercent(line.utilization) }}{{ budgetToneSuffix(line) }}</span>
                </div>
                <div class="list-row__amount">
                  <strong>{{ formatMoney(line.remaining) }}</strong>
                  <span>剩余 / {{ formatMoney(line.planned) }}</span>
                </div>
              </article>
            </div>
            <p v-else class="empty-state">本月尚未设置预算计划。</p>
          </section>

          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">DEBT</p>
                <h2>债务</h2>
              </div>
              <strong>{{ formatMoney(data.debts.total) }}</strong>
            </div>
            <div v-if="data.debts.items.length" class="stack-list">
              <article v-for="debt in data.debts.items" :key="debt.id" class="list-row">
                <div>
                  <strong>{{ debt.name }}</strong>
                  <span>{{ debtTypeLabel(debt.type) }} · {{ repaymentLabel(debt.repayment_type) }} · 每月最低 {{ formatMoney(debt.minimum_payment) }}</span>
                </div>
                <div class="list-row__amount">
                  <strong>{{ formatMoney(debt.balance) }}</strong>
                  <span v-if="debt.apr">APR {{ formatPercent(debt.apr, 2) }}</span>
                </div>
              </article>
            </div>
            <p v-else class="empty-state">没有活动债务。</p>
          </section>

          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">GOALS</p>
                <h2>家庭目标</h2>
              </div>
              <span class="subtle">{{ data.goals.items.length }} 项</span>
            </div>
            <div v-if="data.goals.items.length" class="stack-list">
              <article v-for="goal in data.goals.items" :key="goal.id" class="list-row">
                <div>
                  <strong>{{ goal.name }}</strong>
                  <span>{{ goalStatus(goal.status) }} · 目标日 {{ goal.target_date }}</span>
                </div>
                <div class="list-row__amount">
                  <strong>{{ formatMoney(goal.funded) }} / {{ formatMoney(goal.target) }}</strong>
                  <span>建议每月 {{ formatMoney(goal.required_monthly) }}</span>
                </div>
              </article>
            </div>
            <p v-else class="empty-state">还没有家庭财务目标。</p>
          </section>
        </div>

        <PortfolioPanel
          :default-currency="data.overview.net_worth.currency"
          :read-only="authState.role === 'viewer'"
        />

        <HouseholdMembersPanel v-if="authState.role === 'owner'" />

        <section class="panel advisor-panel">
          <div class="panel-heading">
            <div>
              <p class="eyebrow">AI ADVISOR</p>
              <h2>问 Finance Advisor</h2>
            </div>
            <span class="subtle">重大问题默认请求 Reviewer</span>
          </div>
          <form class="advisor-form" @submit.prevent="submitAdvisor">
            <textarea v-model="advisorQuestion" ref="advisorInput" maxlength="8192" rows="3" aria-label="向 AI 顾问提问" placeholder="例如：现在买一台 8999 元的电脑，会影响应急资金和还债计划吗？" @keydown="onAdvisorKeydown" />
            <button type="submit" :disabled="advisorLoading || !advisorQuestion.trim()">{{ advisorLoading ? '分析中…' : '分析' }}</button>
          </form>
          <div v-if="advisorErrorCode" class="advisor-result advisor-result--failed" role="alert">
            <p>顾问暂时不可用：{{ errorText(advisorErrorCode) }}。可稍后重试；上方指标卡的确定性数字不受影响。</p>
          </div>
          <div v-else-if="advisorResult" class="advisor-result" :class="{ 'advisor-result--blocked': advisorResult.blocked }">
            <p v-if="advisorResult.blocked">本次建议已被安全策略阻止：{{ advisorResult.block_reason }}</p>
            <template v-else>
              <p class="advisor-result__meta">AI 叙述 · 以上方指标卡（确定性计算）为准 · 提问时查看{{ advisorPeriod ? `：${periodLabel(advisorPeriod)}` : '' }}</p>
              <div class="advisor-result__text">{{ advisorResult.text }}</div>
            </template>
            <div v-if="advisorResult.reviewed && advisorResult.review" class="review-box">
              <strong>Reviewer</strong>
              <p>{{ advisorResult.review }}</p>
            </div>
          </div>
        </section>
      </template>

      <section v-else class="panel loading-panel" aria-live="polite">
        {{ loading ? '正在读取家庭财务数据…' : '尚未加载财务数据。' }}
      </section>
    </main>
  </div>

  <ConfirmDialog />
</template>

<style scoped>
.session-check {
  min-height: 100vh;
  display: grid;
  place-items: center;
  text-align: center;
  color: var(--muted);
}

.session-check strong {
  display: block;
  margin-top: 0.45rem;
  color: var(--text);
  font-size: 1.05rem;
}

.topbar-actions {
  display: flex;
  align-items: end;
  justify-content: flex-end;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.signed-in-user {
  align-self: center;
  color: var(--muted);
  font-size: 0.8rem;
}

/* 「本月」复用 .button-link 的文字链接观感;其负 margin 是为行内文本行补偿触控区,
   在步进器下方会与控件重叠,这里还原为普通行内位置 */
.today-button {
  min-height: auto;
  margin: 0;
  padding: 0.25rem 0.85rem 0.25rem 0;
  justify-self: start;
}

.loading-panel {
  text-align: center;
  color: var(--muted);
}

@media (max-width: 719.98px) {
  /* 与 style.css 的 min-width:720px 配对(719.98 避免亚像素重叠) */
  .topbar-actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
