<script setup lang="ts">
import { nextTick, onUnmounted, ref, watch } from 'vue'
import { confirmDialogControls, useConfirm } from '../confirm'

const { state } = useConfirm()
const { cancel, accept, loading } = confirmDialogControls()

const panelRef = ref<HTMLElement | null>(null)
const confirmButtonRef = ref<HTMLButtonElement | null>(null)

// 焦点陷阱:Tab 循环限制在对话框内;打开时焦点落确认按钮(主操作)
function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    event.preventDefault()
    cancel()
    return
  }
  if (event.key !== 'Tab') return
  const focusables = panelRef.value?.querySelectorAll<HTMLElement>('button')
  if (!focusables || focusables.length === 0) return
  const first = focusables[0]
  const last = focusables[focusables.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

watch(
  () => state.visible,
  async (visible) => {
    if (visible) {
      document.addEventListener('keydown', onKeydown, true)
      await nextTick()
      confirmButtonRef.value?.focus()
    } else {
      document.removeEventListener('keydown', onKeydown, true)
    }
  },
)

onUnmounted(() => document.removeEventListener('keydown', onKeydown, true))
</script>

<template>
  <Teleport to="body">
    <div v-if="state.visible" class="confirm-overlay" @click.self="cancel">
      <div
        ref="panelRef"
        class="confirm-dialog"
        role="alertdialog"
        aria-modal="true"
        :aria-label="state.options.title"
      >
        <h3 class="confirm-dialog__title">{{ state.options.title }}</h3>
        <p v-if="state.options.body" class="confirm-dialog__body">{{ state.options.body }}</p>
        <div class="confirm-dialog__actions">
          <button type="button" class="button-secondary" :disabled="loading" @click="cancel">
            {{ state.options.cancelLabel ?? '取消' }}
          </button>
          <button
            ref="confirmButtonRef"
            type="button"
            :class="state.options.danger ? 'button-danger' : ''"
            :disabled="loading"
            @click="accept()"
          >
            {{ loading ? '处理中…' : state.options.confirmLabel ?? '确认' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* 遮罩:墨色轻纱,不抢戏;对话框是系统唯一模态面,沿用环境阴影 */
.confirm-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: grid;
  place-items: center;
  padding: 1rem;
  background: rgb(23 32 51 / 32%);
}

.confirm-dialog {
  width: min(26rem, 100%);
  border: 1px solid var(--border);
  border-radius: 16px;
  background: var(--surface);
  box-shadow: var(--shadow);
  padding: 1.1rem 1.2rem;
}

.confirm-dialog__title {
  margin: 0 0 0.45rem;
  font-size: 1.03rem;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--text);
}

.confirm-dialog__body {
  margin: 0 0 0.9rem;
  color: var(--muted);
  font-size: 0.85rem;
  line-height: 1.55;
  white-space: pre-wrap;
}

.confirm-dialog__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.65rem;
}

.confirm-dialog__actions button {
  min-width: 6rem;
}
</style>
