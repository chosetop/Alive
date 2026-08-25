<script setup lang="ts">
import { THEMES, useTheme, type ThemeName } from '~/composables/useTheme'

/**
 * Theme switch, cycling through the available themes.
 *
 * A cycling button rather than a dropdown: with two themes a menu is heavier
 * than the choice it offers. When the admin gains theme configuration and the
 * list grows past three, this should become a real menu.
 */

const { theme, setTheme } = useTheme()

const LABELS: Record<ThemeName, string> = {
  ink: '墨',
  lamp: '灯',
}

const nextTheme = computed<ThemeName>(() => {
  const index = THEMES.indexOf(theme.value)
  return THEMES[(index + 1) % THEMES.length] ?? 'ink'
})
</script>

<template>
  <button
    class="toggle"
    type="button"
    :aria-label="`切换到${LABELS[nextTheme]}主题`"
    @click="setTheme(nextTheme)"
  >
    <!--
      The label shows the *current* theme, while the action switches to the next.
      aria-label above states the action, since the visible glyph states the
      state.
    -->
    <span aria-hidden="true">{{ LABELS[theme] }}</span>
  </button>
</template>

<style scoped>
.toggle {
  width: 1.75rem;
  height: 1.75rem;
  padding: 0;
  border: 1px solid var(--c-line);
  border-radius: 50%;
  background: transparent;
  color: var(--c-ink-muted);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  line-height: 1;
  cursor: pointer;
  transition:
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.toggle:hover {
  border-color: var(--c-accent);
  color: var(--c-accent);
}
</style>
