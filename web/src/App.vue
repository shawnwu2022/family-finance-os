<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { askAdvisor, loadDashboard } from './api'
import CashflowChart from './components/CashflowChart.vue'
import MetricCard from './components/MetricCard.vue'
import PortfolioPanel from './components/PortfolioPanel.vue'
import { chartValue, formatMoney, formatPercent } from './money'
import type { AdvisorResponse, DashboardData } from './types'

const householdId = ref(localStorage.getItem('finance.household_id') ?? '1')
const period = ref(currentPeriod())
const data = ref<DashboardData | null>(null)
const loading = ref(false)
const errorCode = ref('')
const advisorQuestion = ref('')
const advisorResult = ref<AdvisorResponse | null>(null)
const advisorLoading = ref(false)

const householdNumeric = computed(() => Number.parseInt(householdId.value, 10))
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

async function refresh() {
  const id = householdNumeric.value
  if (!Number.isSafeInteger(id) || id <= 0 || !/^\d{4}-\d{2}$/.test(period.value)) {
    errorCode.value = 'invalid_request'
    return
  }

  loading.value = true
  errorCode.value = ''
  try {
    data.value = await loadDashboard(id, period.value)
    localStorage.setItem('finance.household_id', String(id))
  } catch (error) {
    errorCode.value = error instanceof Error ? error.message : 'request_failed'
  } finally {
    loading.value = false
  }
}

async function submitAdvisor() {
  const id = householdNumeric.value
  const question = advisorQuestion.value.trim()
  if (!Number.isSafeInteger(id) || id <= 0 || !question) return

  advisorLoading.value = true
  advisorResult.value = null
  try {
    advisorResult.value = await askAdvisor(id, question, true)
  } catch (error) {
    advisorResult.value = {
      blocked: true,
      reviewed: false,
      block_reason: error instanceof Error ? error.message : 'request_failed',
    }
  } finally {
    advisorLoading.value = false
  }
}

function currentPeriod(): string {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
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

onMounted(() => {
  void refresh()
})
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <div>
        <p class="eyebrow">FAMILY FINANCE OS</p>
        <h1>家庭财务总览</h1>
        <p class="subtle">确定性计算为准，AI 只负责解释与规划。</p>
      </div>
      <form class="context-controls" @submit.prevent="refresh">
        <label>
          <span>家庭 ID</span>
          <input v-model="householdId" inputmode="numeric" pattern="[0-9]*" aria-label="家庭 ID" />
        </label>
        <label>
          <span>月份</span>
          <input v-model="period" type="month" aria-label="月份" />
        </label>
        <button type="submit" :disabled="loading">{{ loading ? '同步中…' : '刷新' }}</button>
      </form>
    </header>

    <main>
      <div v-if="errorCode" class="notice notice--error" role="alert">
        数据加载失败：{{ errorCode }}。请确认家庭 ID、账本同步和 Finance Core 状态。
      </div>

      <template v-if="data">
        <section class="status-row" aria-label="数据状态">
          <span class="status-pill" :data-quality="data.overview.quality">{{ qualityLabel(data.overview.quality) }}</span>
          <span>数据截至 {{ new Date(data.overview.data_as_of).toLocaleString('zh-CN') }}</span>
        </section>

        <section class="metric-grid" aria-label="核心指标">
          <MetricCard label="安全可消费" :value="formatMoney(data.overview.safe_to_spend)" hint="扣除必要承诺与安全底线后" emphasis />
          <MetricCard label="家庭净资产" :value="formatMoney(data.overview.net_worth)" />
          <MetricCard label="本月净现金流" :value="formatMoney(data.overview.net_cashflow)" :hint="`储蓄率 ${formatPercent(data.overview.savings_rate)}`" />
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
                <h2>本月现金流</h2>
              </div>
              <span class="subtle">{{ period }}</span>
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
              <article v-for="line in data.budget.lines" :key="`${line.kind}:${line.external_category_ref ?? line.semantic_group}`" class="list-row">
                <div>
                  <strong>{{ line.semantic_group || line.external_category_ref || line.kind }}</strong>
                  <span>{{ line.kind }} · 使用 {{ formatPercent(line.utilization) }}</span>
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
                  <span>{{ debt.type }} · {{ debt.repayment_type }} · 每月最低 {{ formatMoney(debt.minimum_payment) }}</span>
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

        <PortfolioPanel :household-id="householdNumeric" :default-currency="data.overview.net_worth.currency" />

        <section class="panel advisor-panel">
          <div class="panel-heading">
            <div>
              <p class="eyebrow">AI ADVISOR</p>
              <h2>问 Finance Advisor</h2>
            </div>
            <span class="subtle">重大问题默认请求 Reviewer</span>
          </div>
          <form class="advisor-form" @submit.prevent="submitAdvisor">
            <textarea v-model="advisorQuestion" maxlength="8192" rows="3" placeholder="例如：现在买一台 8999 元的电脑，会影响应急资金和还债计划吗？" />
            <button type="submit" :disabled="advisorLoading || !advisorQuestion.trim()">{{ advisorLoading ? '分析中…' : '分析' }}</button>
          </form>
          <div v-if="advisorResult" class="advisor-result" :class="{ 'advisor-result--blocked': advisorResult.blocked }">
            <p v-if="advisorResult.blocked">本次建议已被安全策略阻止：{{ advisorResult.block_reason }}</p>
            <p v-else>{{ advisorResult.text }}</p>
            <div v-if="advisorResult.reviewed && advisorResult.review" class="review-box">
              <strong>Reviewer</strong>
              <p>{{ advisorResult.review }}</p>
            </div>
          </div>
        </section>
      </template>

      <section v-else-if="loading" class="loading-state" aria-live="polite">正在读取家庭财务数据…</section>
    </main>
  </div>
</template>
