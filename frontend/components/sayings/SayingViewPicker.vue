<script setup lang="ts">
import { computed } from 'vue'
import type { SayingView } from '~/types'
import PopoverToggle from '~/components/PopoverToggle.vue'

const props = defineProps<{ view: SayingView }>()

const emit = defineEmits<{
  change: [view: SayingView]
}>()

const options: Array<{ view: SayingView; label: string }> = [
  { view: 'stream', label: '流式' },
  { view: 'wall', label: '纸片墙' },
  { view: 'focus', label: '沉浸' },
]

const currentLabel = computed(() => options.find((option) => option.view === props.view)?.label ?? '流式')

function select(value: SayingView, close: () => void): void {
  emit('change', value)
  close()
}
</script>

<template>
  <PopoverToggle :label="`当前浏览方式：${currentLabel}`">
    <template #trigger>{{ currentLabel }}</template>
    <template #menu="{ close }">
      <button
        v-for="option in options"
        :key="option.view"
        type="button"
        role="menuitemradio"
        :aria-checked="view === option.view"
        @click="select(option.view, close)"
      >
        {{ option.label }}
      </button>
    </template>
  </PopoverToggle>
</template>
