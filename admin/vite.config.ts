import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    // Pinned, not left to Vite's fallback. The backend's CORS_ALLOWED_ORIGINS
    // lists this exact origin, and a credentialed API cannot use "*", so a
    // shifted port would fail preflight with nothing in the UI to explain it.
    port: 5173,
    strictPort: true,
  },
})
