// 应用内确认框:替代 window.confirm——原生弹窗是全页唯一浏览器原生元素,调性断裂
// 单例 promise 模式:任意处 await confirmAction(),同一时刻只显示一个对话框
import { readonly, ref } from 'vue'

export interface ConfirmOptions {
  title: string
  body?: string
  // confirmLabel 默认「确认」;danger=true 时按钮走两步确认样式(描边→hover 填充)
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
}

const visible = ref(false)
const options = ref<Readonly<ConfirmOptions>>({ title: '' })
const loading = ref(false)

let activeResolve: ((confirmed: boolean) => void) | null = null

export function confirmAction(newOptions: ConfirmOptions): Promise<boolean> {
  // 已有对话框在展示时直接取消它(理论不可达:调用方 busy 状态互斥,防御性兜底)
  if (activeResolve) {
    activeResolve(false)
    activeResolve = null
  }
  options.value = newOptions
  visible.value = true
  loading.value = false
  return new Promise((resolve) => {
    activeResolve = resolve
  })
}

function settle(result: boolean): void {
  if (!activeResolve) return
  visible.value = false
  loading.value = false
  const resolve = activeResolve
  activeResolve = null
  resolve(result)
}

// Esc 与取消按钮共用;请求进行中不允许关闭
function cancel(): void {
  if (loading.value) return
  settle(false)
}

// 确认后进入 loading:防止双击重复提交,也阻止 Esc/遮罩关闭造成"看似取消实则已提交"
async function accept(action?: () => Promise<void>): Promise<void> {
  if (loading.value) return
  if (action) {
    loading.value = true
    try {
      await action()
    } finally {
      loading.value = false
    }
  }
  settle(true)
}

export function useConfirm() {
  return {
    confirmAction,
    state: readonly({ visible, options, loading }),
  }
}

// 供 ConfirmDialog 组件内部驱动按钮/Esc
export function confirmDialogControls() {
  return { cancel, accept, loading }
}
