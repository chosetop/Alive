import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    // The API client refuses to load without a base URL, by design: a missing
    // one would otherwise surface as every request 404ing against the dev
    // server's own origin. Tests that exercise the real client rather than a
    // module mock need it defined, and a fixed value here keeps assertions about
    // outgoing URLs independent of whatever a developer has in .env.
    env: {
      VITE_API_BASE_URL: 'http://api.test',
    },
  },
})
