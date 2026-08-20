import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { describe, it } from 'node:test'

const root = new URL('./', import.meta.url)

describe('portfolio panel integration', () => {
  it('provides snapshot CRUD without client-side allocation math', async () => {
    const component = await readFile(new URL('components/PortfolioPanel.vue', root), 'utf8')

    for (const token of [
      'listPortfolioAssets',
      'upsertPortfolioAsset',
      'deletePortfolioAsset',
      'buildPortfolioAssetRequest',
      'formFromPortfolioAsset',
      'formatMoney',
    ]) {
      assert.match(component, new RegExp(`\\b${token}\\b`), token)
    }
    assert.doesNotMatch(component, /chartValue\s*\(/)
    assert.doesNotMatch(component, /Number\s*\([^)]*value_minor/)
    assert.match(component, /确认删除/)
    assert.match(component, /资产快照/)
  })

  it('mounts the panel inside the existing dashboard context', async () => {
    const app = await readFile(new URL('App.vue', root), 'utf8')
    assert.match(app, /import PortfolioPanel from ['"]\.\/components\/PortfolioPanel\.vue['"]/)
    assert.match(app, /<PortfolioPanel/)
    assert.match(app, /:household-id="householdNumeric"/)
    assert.match(app, /:default-currency="data\.overview\.net_worth\.currency"/)
  })
})
