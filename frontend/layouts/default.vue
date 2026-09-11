<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useTheme } from '~/composables/useTheme'
import { buildWorldNavigation } from '~/utils/world-navigation'

/**
 * The site frame: masthead, content and a quiet colophon. Category navigation
 * belongs to each world, where readers can reach it before a long archive.
 */

const { data: siteSettings } = useSiteSettings()
const { applyTheme } = useTheme(computed(() => siteSettings.value.default_theme))
applyTheme()

const { data: worldSettings } = await useSiteWorlds()
const navWorlds = computed(() => buildWorldNavigation(worldSettings.value))

const isScrolled = ref(false)
function updateScrollState() {
  isScrolled.value = window.scrollY > 2
}
onMounted(() => {
  updateScrollState()
  window.addEventListener('scroll', updateScrollState, { passive: true })
})
onBeforeUnmount(() => {
  window.removeEventListener('scroll', updateScrollState)
})

const year = new Date().getFullYear()
</script>

<template>
  <div class="site-frame">
    <!-- Keyboard and screen-reader users skip the masthead on every page. -->
    <a class="skip" href="#content">跳到正文</a>

    <header class="masthead" :class="{ 'is-scrolled': isScrolled }">
      <div class="masthead-inner">
        <NuxtLink to="/" class="brand" aria-label="Alive，返回首页" title="返回首页">
          <span class="brand-mark" aria-hidden="true">·</span>
          <span>Alive</span>
        </NuxtLink>
        <nav class="world-nav" aria-label="内容世界">
          <NuxtLink
            v-for="world in navWorlds"
            :key="world.key"
            :to="world.rootPath"
            class="world-link"
          >
            {{ world.label }}
          </NuxtLink>
        </nav>
        <div class="theme-slot">
          <ThemeToggle />
        </div>
      </div>
    </header>

    <div class="shell">
      <main id="content" class="main">
        <slot />
      </main>

      <footer class="footer">
        <small class="copy">&copy; {{ year }} Alive</small>
      </footer>
    </div>
  </div>
</template>

<style scoped>
:global(:root) {
  --alive-masthead-height: calc(5.25rem + env(safe-area-inset-top));
}

.site-frame {
  display: flex;
  flex-direction: column;
  min-height: 100dvh;
}

.shell {
  display: flex;
  flex-direction: column;
  flex: 1;
  width: 100%;
  max-width: var(--page-width);
  margin-inline: auto;
  padding-inline: var(--space-5);
}

/* Visible only when focused, which is the point. */
.skip {
  position: absolute;
  left: -9999px;
}

.skip:focus {
  position: static;
  display: inline-block;
  padding: var(--space-2) 0;
  font-family: var(--font-ui);
  font-size: var(--text-sm);
}

/* ---------------------------------------------------------------- masthead */

.masthead {
  --nav-glass-fallback: var(--c-paper);
  --nav-glass-bg: color-mix(in srgb, var(--c-paper) 78%, transparent);
  --nav-glass-border: color-mix(in srgb, var(--c-line) 72%, transparent);
  --nav-glass-highlight: color-mix(in srgb, white 16%, transparent);
  --nav-glass-shadow: 0 0.5rem 1.25rem color-mix(in srgb, var(--c-shadow-float) 34%, transparent);
  position: sticky;
  top: 0;
  z-index: 40;
  isolation: isolate;
  width: 100%;
  padding-top: env(safe-area-inset-top);
  border-bottom: 1px solid var(--nav-glass-border);
  background: var(--nav-glass-fallback);
  box-shadow: none;
  -webkit-backdrop-filter: blur(20px) saturate(145%);
  backdrop-filter: blur(20px) saturate(145%);
  transition: box-shadow var(--duration-mid) var(--ease-out);
}

.masthead.is-scrolled {
  box-shadow: var(--nav-glass-shadow);
}

.masthead::after {
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  z-index: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--nav-glass-highlight) 18%, var(--nav-glass-highlight) 82%, transparent);
  content: '';
  pointer-events: none;
}

.masthead-inner {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: var(--space-6);
  width: min(100%, var(--page-width));
  min-height: 5.25rem;
  margin-inline: auto;
  padding: 0 var(--space-5);
}

/*
 * The brand is the one place the typeface is shown at display size, which is
 * where a brush face earns its cost. Tight tracking keeps the Latin letterforms
 * from drifting apart at this size.
 */
.brand {
  display: inline-flex;
  align-items: center;
  justify-self: start;
  gap: 0.42rem;
  font-size: var(--text-xl);
  line-height: var(--leading-none);
  letter-spacing: var(--tracking-display);
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out);
}

.brand-mark {
  display: inline-grid;
  width: 0.7rem;
  height: 0.7rem;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--c-accent) 70%, var(--c-line));
  border-radius: 50%;
  color: var(--c-accent);
  font-family: var(--font-ui);
  font-size: 1rem;
  line-height: 0.45;
  transition: background-color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out);
}

.brand::after {
  display: inline-block;
  margin-left: 0.35em;
  color: var(--c-accent);
  content: '↗';
  font-family: var(--font-ui);
  font-size: 0.5em;
  opacity: 0;
  transform: translate(-0.2em, 0.2em);
  transition: opacity var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out);
}

.brand:hover,
.brand:focus-visible {
  color: var(--c-accent);
  transform: translateX(-0.1rem);
}

.brand:hover .brand-mark,
.brand:focus-visible .brand-mark {
  background: color-mix(in srgb, var(--c-accent) 16%, transparent);
  transform: scale(1.12);
}

.brand:hover::after,
.brand:focus-visible::after {
  opacity: 0.8;
  transform: translate(0, 0);
}

.world-nav {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: var(--space-3);
  align-items: center;
  min-width: 0;
}

.theme-slot {
  justify-self: end;
}

.world-link {
  position: relative;
  padding: var(--space-2) var(--space-1);
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out);
}

.world-link:hover,
.world-link:focus-visible,
.world-link.router-link-active {
  color: var(--c-accent);
}

.world-link:hover,
.world-link:focus-visible {
  transform: translateY(-1px);
}

.world-link.router-link-active::after {
  position: absolute;
  right: var(--space-1);
  bottom: 0;
  left: var(--space-1);
  height: 2px;
  border-radius: 999px;
  background: var(--c-accent);
  content: '';
}

.main {
  flex: 1;
  padding-bottom: var(--space-9);
}

/* The fixed player is a floating surface, so the reading column needs to
 * reserve its footprint only while music is available. This keeps the footer
 * reachable without adding an empty gap on pages without a player. */
.site-frame:has(~ .music-player) .main {
  padding-bottom: calc(var(--space-9) + 5.75rem + env(safe-area-inset-bottom));
}

/* ------------------------------------------------------------------ footer */

.footer {
  padding-block: var(--space-6);
  border-top: 1px solid var(--c-line);
}

.site-frame:has(~ .music-player) .footer {
  padding-bottom: calc(var(--space-6) + 5.75rem + env(safe-area-inset-bottom));
}

.copy {
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
}

@media (max-width: 34rem) {
  :global(:root) {
    --alive-masthead-height: calc(4.5rem + env(safe-area-inset-top));
  }

  .masthead-inner {
    grid-template-columns: auto 1fr auto;
    min-height: 4.5rem;
    gap: var(--space-3);
    padding-inline: var(--space-4);
  }

  .main {
    padding-bottom: var(--space-7);
  }

  .world-nav {
    gap: var(--space-2);
  }

  .world-link {
    font-size: var(--text-xs);
  }
}

@supports (backdrop-filter: blur(1px)) {
  .masthead {
    background: var(--nav-glass-bg);
  }
}

@media (prefers-reduced-transparency: reduce), (forced-colors: active) {
  .masthead {
    background: var(--nav-glass-fallback);
    -webkit-backdrop-filter: none;
    backdrop-filter: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .brand,
  .brand::after,
  .brand-mark,
  .masthead {
    transition: none;
  }
}
</style>
