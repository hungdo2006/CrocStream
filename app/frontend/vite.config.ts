import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { randomBytes } from 'node:crypto'

export default defineConfig(({ command }) => {
  const nonce = command === 'serve' ? randomBytes(16).toString('base64') : undefined
  return {
    plugins: [
      react(),
      {
        name: 'dev-csp-nonce',
        transformIndexHtml: (html: string) => nonce
          ? html.replace("script-src 'self'", `script-src 'self' 'nonce-${nonce}'`)
          : html,
      },
    ],
    html: { cspNonce: nonce },
    server: { host: '127.0.0.1', port: 9245, strictPort: true, allowedHosts: ['wails.localhost'] },
    build: { sourcemap: false },
  }
})
