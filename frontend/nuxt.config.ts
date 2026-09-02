// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },

  modules: ['@pinia/nuxt', '@nuxt/eslint'],

  // Components are grouped by domain folders, but their template names stay
  // short (SayingStream, VideoCard, etc.). Without disabling the folder prefix,
  // Nuxt generates names such as SayingsSayingStream and silently leaves the
  // public world pages empty in the browser after hydration.
  components: [{ path: '~/components', pathPrefix: false }],

  // prose.css is global rather than scoped to the page that renders Markdown:
  // scoped styles cannot reach v-html content, since that HTML carries none of
  // the component's generated attributes.
  css: ['@alive/theme/themes.css', '~/assets/css/main.css', '~/assets/css/prose.css'],

  nitro: {
    routeRules: {
      /*
       * The ChillKai webfont, served through this origin.
       *
       * The CDN it lives on returns no Access-Control-Allow-Origin. @font-face
       * fetches are always CORS-mode, so a direct cross-origin url() is fetched,
       * then discarded by the browser: 3.2 MB downloaded and the page still
       * renders in the fallback face. Verified against the CDN on 2026-08-25 with
       * GET, HEAD and an OPTIONS preflight; it sends Timing-Allow-Origin and
       * nothing else.
       *
       * Proxying makes the request same-origin, where CORS does not apply. The
       * cost is that the font passes through the Nuxt server; the long
       * immutable cache below means a given client pays that once.
       */
      '/fonts/chillkai.woff2': {
        proxy:
          'https://mp-af7fce00-5123-404b-a663-efef7a2e5985.cdn.bspapp.com/zine/ChillKai-subset.woff2',
        headers: {
          'cache-control': 'public, max-age=31536000, immutable',
          'content-type': 'font/woff2',
        },
      },
    },
  },

  typescript: {
    strict: true,
    typeCheck: false,
  },

  runtimeConfig: {
    // Container deployments use the service-network hostname during SSR.
    // This value is private and is never serialized into the browser bundle.
    apiInternalBase: '',
    public: {
      // Overridden at runtime by NUXT_PUBLIC_API_BASE.
      // Empty means same-origin in a production browser. The API resolver still
      // maps the direct Nuxt development port to the local Go API.
      apiBase: '',
    },
  },
})
