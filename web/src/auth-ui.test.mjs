import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { describe, it } from 'node:test'

const root = new URL('./', import.meta.url)

async function componentSource(name) {
  try {
    return await readFile(new URL(`components/${name}`, root), 'utf8')
  } catch {
    assert.fail(`${name} must exist`)
  }
}

describe('finance authentication UI', () => {
  it('gates the dashboard behind application auth and removes editable household identity', async () => {
    const app = await readFile(new URL('App.vue', root), 'utf8')

    assert.match(app, /bootstrapSession/)
    assert.match(app, /subscribeAuthState/)
    assert.match(app, /LoginPanel/)
    assert.match(app, /TOTPPanel/)
    assert.match(app, /logout/)
    assert.doesNotMatch(app, /finance\.household_id/)
    assert.doesNotMatch(app, /家庭 ID/)
    assert.doesNotMatch(app, /householdNumeric/)
    assert.doesNotMatch(app, /:household-id=/)
  })

  it('provides a dedicated username/password login form', async () => {
    const login = await componentSource('LoginPanel.vue')
    assert.match(login, /beginLogin/)
    assert.match(login, /autocomplete="username"/)
    assert.match(login, /autocomplete="current-password"/)
    assert.match(login, /登录/)
  })

  it('supports mandatory TOTP enrollment, verification, recovery codes and recovery login', async () => {
    const totp = await componentSource('TOTPPanel.vue')
    for (const token of ['confirmTOTP', 'verifySecondFactor', 'totpSecret', 'otpauthURI', 'recoveryCodes']) {
      assert.match(totp, new RegExp(`\\b${token}\\b`), token)
    }
    assert.match(totp, /恢复码/)
    assert.match(totp, /动态验证码/)
  })
})
