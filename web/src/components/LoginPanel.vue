<script setup lang="ts">
import { ref } from 'vue'
import { beginLogin } from '../auth'

const username = ref('')
const password = ref('')
const busy = ref(false)
const errorMessage = ref('')

async function submit() {
  if (!username.value.trim() || !password.value || busy.value) return
  busy.value = true
  errorMessage.value = ''
  try {
    await beginLogin(username.value, password.value)
    password.value = ''
  } catch (error) {
    const code = error instanceof Error ? error.message : 'request_failed'
    errorMessage.value = code === 'rate_limited'
      ? '尝试次数过多，请稍后再试。'
      : '登录失败，请检查凭据或稍后重试。'
    password.value = ''
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="auth-shell">
    <section class="auth-card" aria-labelledby="finance-login-heading">
      <div class="auth-brand">
        <p class="eyebrow">FAMILY FINANCE OS</p>
        <h1 id="finance-login-heading">登录 Finance</h1>
        <p>家庭财务数据由 Finance Core 自身验证保护。完成密码与双因素验证后才能访问。</p>
      </div>

      <form class="auth-form" @submit.prevent="submit">
        <label class="auth-field">
          <span>用户名</span>
          <input
            v-model.trim="username"
            name="username"
            autocomplete="username"
            autocapitalize="none"
            spellcheck="false"
            required
          />
        </label>
        <label class="auth-field">
          <span>密码</span>
          <input
            v-model="password"
            name="password"
            type="password"
            autocomplete="current-password"
            required
          />
        </label>
        <p v-if="errorMessage" class="auth-error" role="alert">{{ errorMessage }}</p>
        <button type="submit" class="auth-primary" :disabled="busy || !username.trim() || !password">
          {{ busy ? '验证中…' : '登录' }}
        </button>
      </form>
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
  width: min(100%, 430px);
  border: 1px solid var(--border);
  border-radius: 18px;
  background: var(--surface);
  box-shadow: var(--shadow);
  padding: clamp(1.25rem, 4vw, 2rem);
}

.auth-brand h1 {
  margin: 0.2rem 0 0.5rem;
  font-size: clamp(1.7rem, 6vw, 2.25rem);
}

.auth-brand p:last-child {
  margin: 0;
  color: var(--muted);
  line-height: 1.65;
}

.auth-form {
  display: grid;
  gap: 1rem;
  margin-top: 1.5rem;
}

.auth-field {
  display: grid;
  gap: 0.4rem;
  font-size: 0.85rem;
  color: var(--muted);
}

.auth-field input {
  width: 100%;
  min-height: 44px;
  box-sizing: border-box;
}

.auth-error {
  margin: 0;
  border-radius: 10px;
  padding: 0.75rem 0.85rem;
  background: var(--surface-muted);
  color: var(--danger);
  font-size: 0.85rem;
  line-height: 1.5;
}

.auth-primary {
  min-height: 44px;
}
</style>
