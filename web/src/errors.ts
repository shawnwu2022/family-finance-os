// 后端/客户端稳定错误码 → 中文文案;未知码原样保留(可能是自由文本,便于报障)
// 全前端统一出口:App.vue、PortfolioPanel、HouseholdMembersPanel 等
export function errorText(code: string): string {
  const messages: Record<string, string> = {
    invalid_request: '请求参数无效',
    unauthenticated: '登录状态已失效，请重新登录',
    invalid_credentials: '用户名或密码错误',
    invalid_second_factor: '动态验证码不正确',
    invalid_csrf: '安全校验失败，请刷新页面后重试',
    insufficient_role: '当前角色权限不足',
    forbidden: '请求被拒绝',
    rate_limited: '请求过于频繁，请稍后再试',
    not_found: '资源不存在',
    username_exists: '用户名已存在',
    last_owner_required: '至少需要保留一名 Owner',
    method_not_allowed: '请求方式不支持',
    unsupported_media_type: '请求格式不支持',
    internal_error: '服务内部错误',
    request_failed: '网络请求失败',
    logout_failed: '退出登录失败',
  }
  return messages[code] ?? code
}
