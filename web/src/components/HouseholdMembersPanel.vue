<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  createHouseholdMember,
  disableHouseholdMember,
  enableHouseholdMember,
  listHouseholdMembers,
  updateHouseholdMemberRole,
} from '../api'
import { bootstrapSession } from '../auth'
import { confirmAction } from '../confirm'
import { errorText } from '../errors'
import type { HouseholdMemberResponse, HouseholdRole } from '../types'

const items = ref<HouseholdMemberResponse[]>([])
const loading = ref(false)
const saving = ref(false)
const busyUserId = ref<number | null>(null)
const errorCode = ref('')
const form = reactive<{ username: string; password: string; role: HouseholdRole }>({
  username: '',
  password: '',
  role: 'viewer',
})

const roles: Array<{ value: HouseholdRole; label: string }> = [
  { value: 'owner', label: 'Owner · 管理员' },
  { value: 'editor', label: 'Editor · 可编辑' },
  { value: 'viewer', label: 'Viewer · 只读' },
]

async function reload(preserveError = false) {
  loading.value = true
  if (!preserveError) errorCode.value = ''
  try {
    const response = await listHouseholdMembers()
    items.value = response.items
  } catch (error) {
    const failure = error instanceof Error ? error.message : 'request_failed'
    errorCode.value = failure
    if (failure === 'insufficient_role') await bootstrapSession().catch(() => undefined)
  } finally {
    loading.value = false
  }
}

async function createMember() {
  const username = form.username.trim()
  if (!username || !form.password) return

  saving.value = true
  errorCode.value = ''
  try {
    await createHouseholdMember({ username, password: form.password, role: form.role })
    form.username = ''
    form.password = ''
    form.role = 'viewer'
    await reload()
  } catch (error) {
    errorCode.value = error instanceof Error ? error.message : 'request_failed'
  } finally {
    saving.value = false
  }
}

// select 用 :value 而非 v-model:确认前不落地本地角色,取消时只需拨回 select
async function onRoleChange(member: HouseholdMemberResponse, event: Event) {
  const select = event.target as HTMLSelectElement
  const nextRole = select.value as HouseholdRole
  if (nextRole === member.role) return

  const confirmed = await confirmAction({
    title: '变更成员角色',
    body: `确认将「${member.username}」的角色改为 ${roleLabel(nextRole)}？`,
    confirmLabel: '确认变更',
  })
  if (!confirmed) {
    select.value = member.role
    return
  }

  busyUserId.value = member.user_id
  errorCode.value = ''
  try {
    await updateHouseholdMemberRole(member.user_id, nextRole)
    const session = await bootstrapSession()
    if (session.role === 'owner') await reload()
  } catch (error) {
    const failure = error instanceof Error ? error.message : 'request_failed'
    await reload(true)
    errorCode.value = failure
  } finally {
    busyUserId.value = null
  }
}

async function disableMember(member: HouseholdMemberResponse) {
  if (member.disabled || busyUserId.value !== null) return
  if (!(await confirmAction({
    title: '停用成员',
    body: `确认停用家庭成员「${member.username}」？其现有 Finance 会话将立即失效。`,
    confirmLabel: '停用',
    danger: true,
  }))) return

  busyUserId.value = member.user_id
  errorCode.value = ''
  try {
    await disableHouseholdMember(member.user_id)
    await reload()
  } catch (error) {
    errorCode.value = error instanceof Error ? error.message : 'request_failed'
  } finally {
    busyUserId.value = null
  }
}

async function enableMember(member: HouseholdMemberResponse) {
  if (!member.disabled || busyUserId.value !== null) return
  if (!(await confirmAction({
    title: '恢复成员',
    body: `确认恢复成员「${member.username}」的访问？其需要重新登录。`,
    confirmLabel: '恢复',
  }))) return

  busyUserId.value = member.user_id
  errorCode.value = ''
  try {
    await enableHouseholdMember(member.user_id)
    await reload()
  } catch (error) {
    errorCode.value = error instanceof Error ? error.message : 'request_failed'
  } finally {
    busyUserId.value = null
  }
}

function roleLabel(role: HouseholdRole): string {
  return roles.find((item) => item.value === role)?.label ?? role
}

onMounted(() => {
  void reload()
})
</script>

<template>
  <section class="panel members-panel" aria-labelledby="members-heading">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">HOUSEHOLD ACCESS</p>
        <h2 id="members-heading">家庭成员与权限</h2>
        <p class="members-copy">Owner 管理成员；Editor 可修改财务快照；Viewer 可查看、模拟和使用 Advisor，但不能持久化修改。</p>
      </div>
    </div>

    <div v-if="errorCode" class="members-error" role="alert">
      操作失败：{{ errorText(errorCode) }}
      <button type="button" class="button-link" @click="reload()">重试</button>
    </div>

    <form class="member-create" @submit.prevent="createMember">
      <div class="member-create__grid">
        <label>
          <span>用户名</span>
          <input v-model="form.username" maxlength="128" autocomplete="off" required />
        </label>
        <label>
          <span>初始密码</span>
          <input v-model="form.password" type="password" autocomplete="new-password" required />
        </label>
        <label>
          <span>角色</span>
          <select v-model="form.role">
            <option v-for="role in roles" :key="role.value" :value="role.value">{{ role.label }}</option>
          </select>
        </label>
      </div>
      <div class="member-create__actions">
        <span class="members-copy">新成员首次登录必须完成 TOTP 绑定。</span>
        <button type="submit" :disabled="saving || busyUserId !== null || !form.username.trim() || !form.password">
          {{ saving ? '创建中…' : '新增成员' }}
        </button>
      </div>
    </form>

    <div v-if="loading && !items.length" class="members-empty">正在读取家庭成员…</div>
    <div v-else-if="items.length" class="member-list">
      <article v-for="member in items" :key="member.user_id" class="member-row" :class="{ 'member-row--disabled': member.disabled }">
        <div class="member-identity">
          <strong>{{ member.username }}</strong>
          <span>#{{ member.user_id }} · {{ roleLabel(member.role) }}</span>
          <span>{{ member.disabled ? '已停用' : member.totp_enrolled ? 'TOTP 已绑定' : '等待首次 TOTP 绑定' }}</span>
        </div>
        <div class="member-actions">
          <select
            :value="member.role"
            aria-label="成员角色"
            :disabled="busyUserId !== null"
            @change="onRoleChange(member, $event)"
          >
            <option v-for="role in roles" :key="role.value" :value="role.value">{{ role.label }}</option>
          </select>
          <button
            v-if="member.disabled"
            type="button"
            class="button-secondary"
            :disabled="busyUserId !== null"
            @click="enableMember(member)"
          >
            {{ busyUserId === member.user_id ? '处理中…' : '恢复' }}
          </button>
          <button
            v-else
            type="button"
            class="button-danger"
            :disabled="busyUserId !== null"
            @click="disableMember(member)"
          >
            {{ busyUserId === member.user_id ? '处理中…' : '停用' }}
          </button>
        </div>
      </article>
    </div>
    <p v-else-if="!loading" class="members-empty">当前没有可显示的家庭成员。</p>
  </section>
</template>

<style scoped>
.members-panel {
  margin-top: 0.9rem;
}

.members-copy,
.members-empty,
.member-identity span {
  color: var(--muted);
  font-size: 0.8rem;
  line-height: 1.55;
}

.members-error {
  margin-bottom: 0.8rem;
  border-radius: 10px;
  padding: 0.7rem 0.8rem;
  color: var(--danger);
  background: var(--surface-muted);
  font-size: 0.82rem;
}

.member-create {
  margin-bottom: 0.9rem;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: var(--surface-muted);
  padding: 0.9rem;
}

.member-create__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
}

.member-create__grid label {
  display: grid;
  gap: 0.35rem;
  color: var(--muted);
  font-size: 0.78rem;
}

.member-create__actions,
.member-row,
.member-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.member-create__actions {
  margin-top: 0.8rem;
}

.member-list {
  display: grid;
}

.member-row {
  border-top: 1px solid var(--border);
  padding: 0.8rem 0;
}

.member-row--disabled {
  opacity: 0.62;
}

.member-identity {
  display: grid;
  gap: 0.25rem;
}

.member-actions select {
  min-width: 10rem;
}

@media (max-width: 719.98px) {
  .member-create__grid {
    grid-template-columns: 1fr;
  }

  .member-create__actions,
  .member-row {
    align-items: stretch;
    flex-direction: column;
  }

  .member-actions {
    width: 100%;
  }

  .member-actions select,
  .member-actions button {
    flex: 1;
  }
}
</style>
