<script setup lang="ts">
import {
  SelectContent,
  SelectIcon,
  SelectItem,
  SelectItemIndicator,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
} from 'reka-ui'
import { computed } from 'vue'

export interface UiSelectOption {
  value: string
  label: string
  description?: string
  disabled?: boolean
}

export interface UiSelectProps {
  modelValue: string
  options: UiSelectOption[]
  label: string
  placeholder?: string
  disabled?: boolean
  size?: 'default' | 'compact'
}

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<UiSelectProps>(), {
  placeholder: '请选择',
  disabled: false,
  size: 'default',
})

const EMPTY_VALUE = '__alive_select_empty__'
const internalValue = computed(() => props.modelValue === '' ? EMPTY_VALUE : props.modelValue)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  change: [value: string]
}>()

function select(value: unknown): void {
  if (typeof value !== 'string') return
  const nextValue = value === EMPTY_VALUE ? '' : value
  emit('update:modelValue', nextValue)
  emit('change', nextValue)
}
</script>

<template>
  <SelectRoot :model-value="internalValue" :disabled="disabled" @update:model-value="select">
    <SelectTrigger v-bind="$attrs" class="ui-select__trigger" :data-size="size" :aria-label="label">
      <SelectValue :placeholder="placeholder" />
      <SelectIcon class="ui-select__chevron" aria-hidden="true">
        <svg viewBox="0 0 12 12"><path d="m2.25 4.25 3.75 3.5 3.75-3.5" /></svg>
      </SelectIcon>
    </SelectTrigger>

    <SelectPortal>
      <SelectContent class="ui-select__content" position="popper" :side-offset="6" align="start" :body-lock="false">
        <SelectViewport class="ui-select__viewport">
          <SelectItem
            v-for="option in options"
            :key="option.value"
            class="ui-select__item"
            :value="option.value === '' ? EMPTY_VALUE : option.value"
            :disabled="option.disabled"
          >
            <SelectItemIndicator class="ui-select__indicator" aria-hidden="true">
              <svg viewBox="0 0 12 12"><path d="m2 6.2 2.4 2.3L10 3" /></svg>
            </SelectItemIndicator>
            <span class="ui-select__copy">
              <SelectItemText>{{ option.label }}</SelectItemText>
              <small v-if="option.description">{{ option.description }}</small>
            </span>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>
