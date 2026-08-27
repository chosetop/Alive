<script setup lang="ts">
import { THEMES, type ThemeName } from '@alive/theme'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useTheme } from '~/composables/useTheme'

/**
 * Accessible theme menu. The first option clears the visitor cookie so the
 * reader follows the site's current default.
 */

const { data: siteSettings } = useSiteSettings()
const { theme, visitorTheme, setVisitorTheme } = useTheme(computed(() => siteSettings.value.default_theme))
const toggle = ref<HTMLDetailsElement | null>(null)

const labels: Record<ThemeName, string> = Object.fromEntries(THEMES.map((item) => [item.name, item.label])) as Record<ThemeName, string>

function selectTheme(value: string): void {
  setVisitorTheme(value === 'site' ? null : (value as ThemeName))
  if (toggle.value) toggle.value.open = false
}

function closeOnOutsidePointerDown(event: PointerEvent): void {
  const target = event.target
  if (toggle.value?.open && target instanceof Node && !toggle.value.contains(target)) {
    toggle.value.open = false
  }
}

function closeOnEscape(event: KeyboardEvent): void {
  if (event.key === 'Escape' && toggle.value?.open) {
    toggle.value.open = false
  }
}

function openOnHover(): void {
  if (toggle.value) toggle.value.open = true
}

function closeOnHoverEnd(): void {
  if (toggle.value) toggle.value.open = false
}

onMounted(() => {
  document.addEventListener('pointerdown', closeOnOutsidePointerDown)
  document.addEventListener('keydown', closeOnEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', closeOnOutsidePointerDown)
  document.removeEventListener('keydown', closeOnEscape)
})
</script>

<template>
  <details ref="toggle" class="toggle" @mouseenter="openOnHover" @mouseleave="closeOnHoverEnd">
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

/* Bridges the visual gap so hover can travel from the pill into the popover. */
.toggle::after {
  position: absolute;
  top: 100%;
  right: 0;
  width: 100%;
  height: var(--space-2);
  content: '';
}

summary {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--c-line-strong);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 88%, var(--c-accent));
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-xs);
  cursor: pointer;
  list-style: none;
  transition: border-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
}

summary::-webkit-details-marker {
  display: none;
}

summary::before {
  width: 0.5rem;
  height: 0.5rem;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--c-accent);
  box-shadow: 0 0 0 0.2rem color-mix(in srgb, var(--c-accent) 16%, transparent);
  content: '';
}

summary::after {
  width: 0.35rem;
  height: 0.35rem;
  margin-left: var(--space-1);
  border-right: 1px solid currentColor;
  border-bottom: 1px solid currentColor;
  content: '';
  transform: rotate(45deg) translateY(-0.1rem);
  transition: transform var(--duration-fast) var(--ease-out);
}

summary:hover,
summary:focus-visible,
details[open] summary {
  border-color: var(--c-accent);
  color: var(--c-accent);
  outline: none;
  transform: translateY(-1px);
}

details[open] summary::after {
  transform: rotate(225deg) translate(-0.05rem, -0.05rem);
}

.menu {
  position: absolute;
  top: calc(100% + var(--space-2));
  right: 0;
  z-index: 2;
  display: grid;
  min-width: 11.5rem;
  padding: 0.375rem;
  border: 1px solid color-mix(in srgb, var(--c-line) 72%, transparent);
  border-radius: 0.875rem;
  background: color-mix(in srgb, var(--c-surface) 88%, transparent);
  box-shadow: 0 0.75rem 2rem color-mix(in srgb, var(--c-overlay) 72%, transparent), 0 0.125rem 0.375rem color-mix(in srgb, var(--c-ink) 10%, transparent);
  -webkit-backdrop-filter: saturate(1.4) blur(1rem);
  backdrop-filter: saturate(1.4) blur(1rem);
  animation: theme-menu-in var(--duration-fast) var(--ease-out);
}

@keyframes theme-menu-in {
  from {
    opacity: 0;
    transform: translateY(-0.25rem) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.menu button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  width: 100%;
  padding: var(--space-2) var(--space-3);
  border: 0;
  border-radius: 0.625rem;
  background: transparent;
  color: var(--c-ink);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out);
}

.menu button:hover,
.menu button[aria-checked='true'] {
  background: var(--c-surface-sunken);
  color: var(--c-accent);
}

.menu button[aria-checked='true']::after {
  content: '✓';
  font-family: var(--font-ui);
  font-size: var(--text-xs);
}

.menu button:active {
  transform: scale(0.98);
}

@media (prefers-reduced-motion: reduce) {
  summary,
  summary::after,
  .menu,
  .menu button {
    animation: none;
    transition: none;
  }
}
</style>
