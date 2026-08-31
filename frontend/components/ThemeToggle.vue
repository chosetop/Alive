<script setup lang="ts">
import { THEMES, type ThemeName } from '@alive/theme'
import { useTheme } from '~/composables/useTheme'
import PopoverToggle from '~/components/PopoverToggle.vue'

const { data: siteSettings } = useSiteSettings()
const { theme, visitorTheme, setVisitorTheme } = useTheme(computed(() => siteSettings.value.default_theme))
const labels: Record<ThemeName, string> = Object.fromEntries(THEMES.map((item) => [item.name, item.label])) as Record<ThemeName, string>

function selectTheme(value: string, close: () => void): void {
  setVisitorTheme(value === 'site' ? null : (value as ThemeName))
  close()
}
</script>

<template>
  <PopoverToggle :label="`当前主题：${labels[theme]}`" align="right">
    <template #trigger>{{ labels[theme] }}</template>
    <template #menu="{ close }">
      <button type="button" role="menuitemradio" :aria-checked="visitorTheme === null" @click="selectTheme('site', close)">
        跟随站点
      </button>
      <button
        v-for="item in THEMES"
        :key="item.name"
        type="button"
        role="menuitemradio"
        :aria-checked="theme === item.name"
        @click="selectTheme(item.name, close)"
      >
        {{ item.label }}
      </button>
    </template>
  </PopoverToggle>
</template>
