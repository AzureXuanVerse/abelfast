import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

// The build is embedded by internal/admin (go:embed dist); in dev, the API
// is belfast's -admin listener ($BELFAST_ADMIN, 127.0.0.1:8081 by default).
export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  build: { outDir: '../internal/admin/dist', emptyOutDir: true },
  server: { proxy: { '/api': `http://${process.env.BELFAST_ADMIN ?? '127.0.0.1:8081'}` } },
})
