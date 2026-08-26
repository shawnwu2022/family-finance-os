import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { it } from 'node:test'
import vm from 'node:vm'

it('uses the network for navigations and refreshes the cached shell', async () => {
  const source = await readFile(new URL('../public/sw.js', import.meta.url), 'utf8')
  const listeners = new Map()
  const writes = []
  const stale = new Response('stale shell', { status: 200 })
  const network = new Response('fresh application', { status: 200 })
  const sandbox = {
    URL,
    fetch: async () => network.clone(),
    caches: {
      match: async (request) => (request === '/' || request?.url ? stale.clone() : undefined),
      open: async () => ({
        addAll: async () => {},
        put: async (request, response) => writes.push({ request, body: await response.text() }),
      }),
      keys: async () => [],
      delete: async () => true,
    },
    self: {
      location: { origin: 'https://finance.test' },
      clients: { claim: () => {} },
      skipWaiting: () => {},
      addEventListener: (name, listener) => listeners.set(name, listener),
    },
  }
  vm.runInNewContext(source, sandbox)

  let responsePromise
  listeners.get('fetch')({
    request: { method: 'GET', url: 'https://finance.test/dashboard', mode: 'navigate' },
    respondWith: (value) => { responsePromise = value },
  })

  const response = await responsePromise
  assert.equal(await response.text(), 'fresh application')
  assert.equal(writes.length, 1)
  assert.equal(writes[0].body, 'fresh application')
})
