import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  // Production is mounted below the public site's /admin path. Development
  // keeps the root base so the documented localhost URLs remain unchanged.
  base: process.env.NODE_ENV === 'production' ? '/admin/' : '/',
  server: {
    // Pinned, not left to Vite's fallback. The backend's CORS_ALLOWED_ORIGINS
    // lists this exact origin, and a credentialed API cannot use "*", so a
    // shifted port would fail preflight with nothing in the UI to explain it.
    port: 5173,
    strictPort: true,
    proxy: {
      '/fonts/chillkai.woff2': {
        target: 'https://mp-af7fce00-5123-404b-a663-efef7a2e5985.cdn.bspapp.com',
        changeOrigin: true,
        rewrite: () => '/zine/ChillKai-subset.woff2',
        headers: { referer: 'https://mp-af7fce00-5123-404b-a663-efef7a2e5985.cdn.bspapp.com/' },
      },
    },
  },
})
