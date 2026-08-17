import { access, readFile } from 'node:fs/promises'

const requiredFiles = [
  'index.html',
  'src/main.ts',
  'src/App.vue',
  'public/manifest.webmanifest',
  'public/sw.js',
]

for (const file of requiredFiles) {
  try {
    await access(file)
  } catch {
    throw new Error(`missing required PWA file: ${file}`)
  }
}

const manifest = JSON.parse(await readFile('public/manifest.webmanifest', 'utf8'))
if (!manifest.name || !manifest.short_name || manifest.display !== 'standalone') {
  throw new Error('manifest must define name, short_name, and display=standalone')
}
if (manifest.start_url !== '/' || manifest.scope !== '/') {
  throw new Error('manifest start_url and scope must stay same-origin at /')
}
if (!Array.isArray(manifest.icons) || manifest.icons.length === 0) {
  throw new Error('manifest must include at least one application icon')
}

const worker = await readFile('public/sw.js', 'utf8')
if (!worker.includes("url.pathname.startsWith('/api/')")) {
  throw new Error('service worker must explicitly bypass /api/ requests')
}
if (!worker.includes("cache: 'no-store'")) {
  throw new Error('service worker must fetch /api/ with cache=no-store')
}
if (!worker.includes('caches.open(')) {
  throw new Error('service worker must cache the application shell')
}

const entry = await readFile('src/main.ts', 'utf8')
if (!entry.includes("navigator.serviceWorker.register('/sw.js')")) {
  throw new Error('application must register /sw.js')
}

console.log('PWA contract OK')
