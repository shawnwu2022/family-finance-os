<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  clearRecoveryCodes,
  confirmTOTP,
  verifySecondFactor,
  type AuthState,
} from '../auth'

const props = defineProps<{
  state: AuthState
}>()

const code = ref('')
const useRecovery = ref(false)
const busy = ref(false)
const errorMessage = ref('')

const showingRecoveryCodes = computed(
  () => props.state.authenticated && props.state.recoveryCodes.length > 0,
)
const enrolling = computed(() => props.state.phase === 'enroll_totp')
const verifying = computed(() => props.state.phase === 'verify_totp')
const totpSecret = computed(() => props.state.totpSecret)
const otpauthURI = computed(() => props.state.otpauthURI)
const recoveryCodes = computed(() => props.state.recoveryCodes)

async function submitSecondFactor() {
  const value = code.value.trim()
  if (!value || busy.value) return
  busy.value = true
  errorMessage.value = ''
  try {
    if (enrolling.value) {
      await confirmTOTP(value)
    } else if (verifying.value) {
      await verifySecondFactor(value, useRecovery.value)
    }
    code.value = ''
  } catch (error) {
    const failure = error instanceof Error ? error.message : 'request_failed'
    errorMessage.value = failure === 'rate_limited'
      ? '尝试次数过多，请稍后再试。'
      : '验证失败，请检查验证码后重试。'
    code.value = ''
  } finally {
    busy.value = false
  }
}

function continueAfterRecoveryCodes() {
  clearRecoveryCodes()
}
</script>

<template>
  <main class="auth-shell">
    <section class="auth-card" aria-labelledby="finance-totp-heading">
      <template v-if="showingRecoveryCodes">
        <div class="auth-brand">
          <p class="eyebrow">RECOVERY CODES</p>
          <h1 id="finance-totp-heading">保存恢复码</h1>
          <p>这些恢复码只显示这一次。请保存到密码管理器或其他安全位置，不要截图后留在普通相册。</p>
        </div>
        <ol class="recovery-list" aria-label="一次性恢复码">
          <li v-for="recovery in recoveryCodes" :key="recovery"><code>{{ recovery }}</code></li>
        </ol>
        <button type="button" class="auth-primary" @click="continueAfterRecoveryCodes">我已安全保存，进入财务首页</button>
      </template>

      <template v-else>
        <div class="auth-brand">
          <p class="eyebrow">TWO-FACTOR AUTHENTICATION</p>
          <h1 id="finance-totp-heading">{{ enrolling ? '设置双因素验证' : '双因素验证' }}</h1>
          <p v-if="enrolling">用验证器 App 添加下面的密钥，然后输入当前 6 位动态验证码。完成前不会开放财务数据。</p>
          <p v-else>输入验证器中的当前动态验证码。只有在无法使用验证器时才使用恢复码。</p>
        </div>

        <div v-if="enrolling" class="enrollment-box">
          <span>验证器密钥</span>
          <code class="auth-secret">{{ totpSecret }}</code>
          <details v-if="otpauthURI">
            <summary>显示 otpauth URI</summary>
            <code class="auth-uri">{{ otpauthURI }}</code>
          </details>
        </div>

        <form class="auth-form" @submit.prevent="submitSecondFactor">
          <label class="auth-field">
            <span>{{ useRecovery ? '恢复码' : '动态验证码' }}</span>
            <input
              v-model="code"
              :inputmode="useRecovery ? 'text' : 'numeric'"
              :autocomplete="useRecovery ? 'off' : 'one-time-code'"
              :pattern="useRecovery ? undefined : '[0-9]*'"
              required
            />
          </label>
          <button
            v-if="verifying"
            type="button"
            class="auth-link"
            @click="useRecovery = !useRecovery; code = ''; errorMessage = ''"
          >
            {{ useRecovery ? '改用动态验证码' : '改用恢复码' }}
          </button>
          <p v-if="errorMessage" class="auth-error" role="alert">{{ errorMessage }}</p>
          <button type="submit" class="auth-primary" :disabled="busy || !code.trim()">
            {{ busy ? '验证中…' : enrolling ? '启用并登录' : '验证并登录' }}
          </button>
        </form>
      </template>
    </section>
  </main>
</template>

<style scoped>
.auth-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 1.25rem;
  background: transparent;
}

.auth-card {
  width: min(100%, 520px);
  border: 1px solid var(--border);
  border-radius: 18px;
  background: var(--surface);
  box-shadow: var(--shadow);
  padding: clamp(1.25rem, 4vw, 2rem);
}

.auth-brand h1 {
  margin: 0.2rem 0 0.5rem;
  font-size: clamp(1.6rem, 6vw, 2.15rem);
}

.auth-brand p:last-child {
  margin: 0;
  color: var(--muted);
  line-height: 1.65;
}

.enrollment-box {
  display: grid;
  gap: 0.5rem;
  margin: 1.25rem 0;
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 0.9rem;
  background: var(--surface-muted);
}

.auth-secret,
.auth-uri,
.recovery-list code {
  overflow-wrap: anywhere;
  user-select: all;
}

.auth-secret {
  font-size: 1rem;
  letter-spacing: 0.08em;
}

.auth-uri {
  display: block;
  margin-top: 0.55rem;
  font-size: 0.72rem;
  line-height: 1.45;
}

.auth-form {
  display: grid;
  gap: 0.9rem;
  margin-top: 1.25rem;
}

.auth-field {
  display: grid;
  gap: 0.4rem;
  font-size: 0.85rem;
  color: var(--muted);
}

.auth-field input,
.auth-primary {
  min-height: 44px;
}

.auth-link {
  justify-self: start;
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--accent);
}

.auth-error {
  margin: 0;
  color: var(--danger);
  font-size: 0.85rem;
}

.recovery-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.55rem 1rem;
  margin: 1.25rem 0;
  padding-left: 1.6rem;
}

.recovery-list li {
  padding: 0.35rem 0;
}

@media (max-width: 560px) {
  .recovery-list {
    grid-template-columns: 1fr;
  }
}
</style>
