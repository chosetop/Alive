<script setup lang="ts">
import { THEMES, type ThemeName } from '@alive/theme'
import { useTheme } from '~/composables/useTheme'

/**
 * Accessible theme menu. The first option clears the visitor cookie so the
 * reader follows the site's current default.
 */

const { data: siteSettings } = useSiteSettings()
const { theme, visitorTheme, setVisitorTheme } = useTheme(computed(() => siteSettings.value.default_theme))

const labels: Record<ThemeName, string> = Object.fromEntries(THEMES.map((item) => [item.name, item.label])) as Record<ThemeName, string>

function selectTheme(value: string): void {
  setVisitorTheme(value === 'site' ? null : (value as ThemeName))
}
</script>

<template>
  <details class="toggle">
    <summary :aria-label="`当前主题：${labels[theme]}`">{{ labels[theme] }}</summary>
    <div class="menu" role="menu" aria-label="选择主题">
      <button type="button" role="menuitem" :aria-checked="visitorTheme === null" @click="selectTheme('site')">
        跟随站点
      </button>
      <button
        v-for="item in THEMES"
        :key="item.name"
        type="button"
        role="menuitemradio"
        :aria-checked="theme === item.name"
        @click="selectTheme(item.name)"
      >
        {{ item.label }}
      </button>
    </div>
  </details>
</template>

<style scoped>
.toggle {
  position: relative;
}

summary {
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--c-line);
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
  cursor: pointer;
  list-style: none;
}

.menu {
  position: absolute;
  top: calc(100% + var(--space-2));
  right: 0;
  z-index: 2;
  display: grid;
  min-width: 9rem;
  padding: var(--space-2);
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  box-shadow: 0 8px 20px var(--c-overlay);
}

.menu button {
  padding: var(--space-2) var(--space-3);
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
}

.menu button:hover,
.menu button[aria-checked='true'] {
  background: var(--c-surface-sunken);
  color: var(--c-accent);
}
</style>
