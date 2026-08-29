import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { describe, it } from 'node:test'

const root = new URL('./', import.meta.url)

describe('household role UI', () => {
  it('renders member management only for owners and makes viewer portfolio read-only', async () => {
    const app = await readFile(new URL('App.vue', root), 'utf8')
    const portfolio = await readFile(new URL('components/PortfolioPanel.vue', root), 'utf8')

    assert.match(app, /HouseholdMembersPanel/)
    assert.match(app, /authState\.role === 'owner'/)
    assert.match(app, /:read-only="authState\.role === 'viewer'"/)
    assert.match(portfolio, /readOnly\?: boolean/)
    assert.match(portfolio, /Viewer · 只读/)
    assert.match(portfolio, /v-if="!readOnly"/)
  })

  it('owner panel supports create, role change, disable and first-login TOTP messaging', async () => {
    const panel = await readFile(new URL('components/HouseholdMembersPanel.vue', root), 'utf8')

    for (const token of [
      'createHouseholdMember',
      'updateHouseholdMemberRole',
      'disableHouseholdMember',
      'enableHouseholdMember',
      'listHouseholdMembers',
    ]) {
      assert.match(panel, new RegExp(`\\b${token}\\b`), token)
    }
    assert.match(panel, /首次登录必须完成 TOTP/)
    // 恢复为 CJK token，\b 边界对 CJK 无效，改用字面断言
    assert.match(panel, /恢复/)
    assert.match(panel, /确认恢复成员/)
    assert.match(panel, /Owner · 管理员/)
    assert.match(panel, /Editor · 可编辑/)
    assert.match(panel, /Viewer · 只读/)
    assert.match(panel, /bootstrapSession/)
    assert.match(panel, /reload\(true\)/)
  })
})
