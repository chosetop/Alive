<script setup lang="ts">
import { useTheme } from '~/composables/useTheme'
import { buildWorldNavigation } from '~/utils/world-navigation'

/**
 * The site frame: masthead, content, footer with category navigation.
 *
 * Categories are fetched here rather than per page so the footer is populated
 * everywhere without each page knowing about it. `useAsyncData` with a fixed key
 * means one request per render, shared across navigations.
 */

const { data: siteSettings } = useSiteSettings()
const { applyTheme } = useTheme(computed(() => siteSettings.value.default_theme))
applyTheme()

/**
 * Shared with the category page through one composable, so both use the same
 * cache key *and* the same options. See composables/useSiteCategories.ts.
 */
const { data: categories } = await useSiteCategories()
const { data: worldSettings } = await useSiteWorlds()

/** Empty categories are hidden: a nav link to nothing is a dead end. */
const navCategories = computed(() => categories.value.filter((c) => c.entry_count > 0))
const navWorlds = computed(() => buildWorldNavigation(worldSettings.value))

const year = new Date().getFullYear()
</script>

<template>
  <div class="shell">
    <!-- Keyboard and screen-reader users skip the masthead on every page. -->
    <a class="skip" href="#content">跳到正文</a>

    <header class="masthead">
      <NuxtLink to="/" class="brand" aria-label="Alive，返回首页" title="返回首页">Alive</NuxtLink>
      <div class="masthead-tools">
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
        <ThemeToggle />
      </div>
    </header>

    <main id="content" class="main">
      <slot />
    </main>

    <footer class="footer">
      <nav v-if="navCategories.length > 0" class="cats" aria-label="分类">
        <NuxtLink
          v-for="c in navCategories"
          :key="c.slug"
          :to="`/journal/categories/${c.slug}`"
          class="cat"
        >
          {{ c.name }}
          <span class="count">{{ c.entry_count }}</span>
        </NuxtLink>
      </nav>

      <small class="copy">&copy; {{ year }} Alive</small>
    </footer>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  flex-direction: column;
  min-height: 100dvh;
  /*
   * One column shared by header, main and footer, so the brand, the text and the
   * footer links all sit on the same left edge. Centring each independently is
   * how mastheads end up a few pixels off from the content below them.
   */
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
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding-block: var(--space-6);
}

/*
 * The brand is the one place the typeface is shown at display size, which is
 * where a brush face earns its cost. Tight tracking keeps the Latin letterforms
 * from drifting apart at this size.
 */
.brand {
  position: relative;
  font-size: var(--text-xl);
  line-height: var(--leading-none);
  letter-spacing: var(--tracking-display);
  text-decoration: none;
  transition: color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out);
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

.brand:hover::after,
.brand:focus-visible::after {
  opacity: 0.8;
  transform: translate(0, 0);
}

.masthead-tools {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: var(--space-4);
  min-width: 0;
}

.world-nav {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--space-3);
  align-items: center;
  padding: 0.25rem;
  border: 1px solid transparent;
  border-radius: 999px;
  transition: border-color var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
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

/* ------------------------------------------------------------------ footer */

.footer {
  padding-block: var(--space-6);
  border-top: 1px solid var(--c-line);
}

.cats {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2) var(--space-5);
  margin-bottom: var(--space-5);
}

.cat {
  color: var(--c-ink-muted);
  font-size: var(--text-sm);
  text-decoration: none;
}

.cat:hover {
  color: var(--c-accent);
}

/* Superscript, the way a printed index numbers its entries. */
.count {
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: 0.625rem;
  font-variant-numeric: tabular-nums;
  vertical-align: super;
}

.copy {
  color: var(--c-ink-faint);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
}

@media (max-width: 34rem) {
  .masthead {
    padding-block: var(--space-5);
  }

  .main {
    padding-bottom: var(--space-7);
  }

  .masthead-tools {
    gap: var(--space-2);
  }

  .world-nav {
    gap: var(--space-2);
    padding-inline: 0;
  }

  .world-link {
    font-size: var(--text-xs);
  }
}

@media (prefers-reduced-motion: reduce) {
  .brand,
  .brand::after {
    transition: none;
  }
}
</style>
