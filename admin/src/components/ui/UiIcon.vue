<script setup lang="ts">
/**
 * The sole icon boundary for the admin UI.
 *
 * The registry deliberately holds the small set of workspace actions rather
 * than exposing a package API to business components. New icons are added here
 * with a semantic name, which keeps visual weight and accessibility treatment
 * consistent across the app.
 */
export type UiIconName =
  | 'search'
  | 'chevron-left'
  | 'chevron-right'
  | 'chevron-down'
  | 'more-horizontal'
  | 'plus'
  | 'close'
  | 'check'
  | 'trash'
  | 'external-link'

export interface UiIconProps {
  name: UiIconName
}

defineProps<UiIconProps>()

const paths: Record<UiIconName, string[]> = {
  search: ['m21 21-4.35-4.35', 'a9 9 0 1 1-2.3-2.3L21 21Z'],
  'chevron-left': ['m15 18-6-6 6-6'],
  'chevron-right': ['m9 18 6-6-6-6'],
  'chevron-down': ['m6 9 6 6 6-6'],
  'more-horizontal': ['M5 12h.01', 'M12 12h.01', 'M19 12h.01'],
  plus: ['M12 5v14', 'M5 12h14'],
  close: ['m6 6 12 12', 'm18 6-12 12'],
  check: ['m5 12 4 4L19 6'],
  trash: ['M4 7h16', 'M10 11v6', 'M14 11v6', 'M9 7V4h6v3', 'M6 7l1 14h10l1-14'],
  'external-link': ['M14 5h5v5', 'm19 5-9 9', 'M5 10v9h9'],
}
</script>

<template>
  <svg
    class="ui-icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path v-for="path in paths[name]" :key="path" :d="path" />
  </svg>
</template>
