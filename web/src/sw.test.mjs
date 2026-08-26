import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { it } from 'node:test'
import vm from 'node:vm'

function loadServiceWorker({ fetchImpl, cacheMatch }) {
  return readFile(new URL('../public/sw.js', import.meta.url), 'utf8').then((source) => {
    const listeners = new Map()
    const writes = []
    const fetches = []
    const sandbox = {
      URL,
      fetch: async (request, init) => {
        fetches.push({ request, init })
        return fetchImpl(request, init)
      },
      caches: {
        match: cacheMatch,
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
    return { listeners, writes, fetches }
  })
}

it('uses network for navigations without writing rendered responses into cache', async () => {
  const network = new Response('fresh application', { status: 200 })
  const { listeners, writes } = await loadServiceWorker({
    fetchImpl: async () => network.clone(),
    cacheMatch: async () => new Response('stale shell', { status: 200 }),
  })

  let responsePromise
  listeners.get('fetch')({
    request: { method: 'GET', url: 'https://finance.test/dashboard', mode: 'navigate' },
    respondWith: (value) => { responsePromise = value },
  })

  const response = await responsePromise
  assert.equal(await response.text(), 'fresh application')
  assert.equal(writes.length, 0)
})

it('never caches api or auth responses and forces no-store', async () => {
  const { listeners, writes, fetches } = await loadServiceWorker({
    fetchImpl: async () => new Response(JSON.stringify({ authenticated: true }), { status: 200 }),
    cacheMatch: async () => undefined,
  })

  let responsePromise
  listeners.get('fetch')({
    request: { method: 'GET', url: 'https://finance.test/api/v1/auth/session', mode: 'cors' },
    respondWith: (value) => { responsePromise = value },
  })

  await responsePromise
  assert.equal(writes.length, 0)
  assert.equal(fetches.length, 1)
  assert.equal(fetches[0].init.cache, 'no-store')
})
