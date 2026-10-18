import assert from 'node:assert/strict'
import { after, before, test } from 'node:test'
import { createServer } from 'vite'
import { createSSRApp } from 'vue'
import { renderToString } from 'vue/server-renderer'

let server
let TextComponent

before(async () => {
  server = await createServer({
    server: { middlewareMode: true },
    optimizeDeps: { disabled: true },
    resolve: { alias: [{ find: '@/locales', replacement: '\0test-locales' }] },
    plugins: [{
      name: 'test-locales',
      enforce: 'pre',
      resolveId(id) {
        if (id === '\0test-locales')
          return '\0test-locales'
      },
      load(id) {
        if (id === '\0test-locales')
          return 'export const t = key => key'
      },
    }],
  })
  TextComponent = (await server.ssrLoadModule('/src/views/chat/components/Message/Text.vue')).default
})

after(async () => {
  await server?.close()
})

test('shows a cursor while waiting for the first chunk', async () => {
  const html = await renderToString(createSSRApp(TextComponent, { text: '', loading: true }))
  assert.match(html, /animate-blink/)
  assert.doesNotMatch(html, /font-size:12px/)
})

test('renders streamed Markdown before the answer finishes', async () => {
  for (const text of ['first', 'first **second**']) {
    const html = await renderToString(createSSRApp(TextComponent, { text, loading: true }))
    assert.match(html, /first/)
    if (text.includes('second'))
      assert.match(html, /<strong>second<\/strong>/)
    assert.match(html, /animate-blink/)
    assert.doesNotMatch(html, /font-size:12px/)
  }
})

test('renders streamed raw text without interpreting it as HTML', async () => {
  const html = await renderToString(createSSRApp(TextComponent, {
    text: '<partial>',
    loading: true,
    asRawText: true,
  }))
  assert.match(html, /&lt;partial&gt;/)
  assert.match(html, /animate-blink/)
})

test('keeps completed text and removes the streaming cursor', async () => {
  const html = await renderToString(createSSRApp(TextComponent, {
    text: 'completed **answer**',
    loading: false,
  }))
  assert.match(html, /completed <strong>answer<\/strong>/)
  assert.match(html, /font-size:12px/)
  assert.doesNotMatch(html, /animate-blink/)
})

test('preserves user message rendering', async () => {
  const html = await renderToString(createSSRApp(TextComponent, {
    text: '**user** <message>',
    inversion: true,
    asRawText: true,
  }))
  assert.match(html, /\*\*user\*\* &lt;message&gt;/)
  assert.doesNotMatch(html, /animate-blink|font-size:12px/)
})
