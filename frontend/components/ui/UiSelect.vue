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

defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue: string
  options: UiSelectOption[]
  label: string
  placeholder?: string
  disabled?: boolean
  size?: 'default' | 'compact'
}>(), {
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
  <SelectRoot :model-value="internalValue" :disabled="props.disabled" @update:model-value="select">
    <SelectTrigger v-bind="$attrs" class="alive-select__trigger" :data-size="props.size" :aria-label="props.label">
      <SelectValue :placeholder="props.placeholder" />
      <SelectIcon class="alive-select__chevron" aria-hidden="true">
        <svg viewBox="0 0 12 12"><path d="m2.25 4.25 3.75 3.5 3.75-3.5" /></svg>
      </SelectIcon>
    </SelectTrigger>
    <SelectPortal>
      <SelectContent class="alive-select__content" position="popper" :side-offset="6" align="start" :body-lock="false">
        <SelectViewport class="alive-select__viewport">
          <SelectItem v-for="option in props.options" :key="option.value" class="alive-select__item" :value="option.value === '' ? EMPTY_VALUE : option.value" :disabled="option.disabled">
            <SelectItemIndicator class="alive-select__indicator" aria-hidden="true">
              <svg viewBox="0 0 12 12"><path d="m2 6.2 2.4 2.3L10 3" /></svg>
            </SelectItemIndicator>
            <span class="alive-select__copy">
              <SelectItemText>{{ option.label }}</SelectItemText>
              <small v-if="option.description">{{ option.description }}</small>
            </span>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>

<style>
.alive-select__trigger {
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  width: 100%;
  min-height: 2.75rem;
  padding: 0.5rem 0.75rem 0.5rem 0.9rem;
  border: 1px solid var(--c-line-strong);
  border-radius: 0.75rem;
  background: color-mix(in srgb, var(--c-surface) 92%, var(--c-accent));
  box-shadow: 0 1px 2px var(--c-shadow-control);
  color: var(--c-ink);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  line-height: 1.25;
  text-align: left;
  cursor: pointer;
  transition: border-color var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out), box-shadow var(--duration-fast) var(--ease-out);
}
.alive-select__trigger[data-size='compact'] { min-height: 2.25rem; padding-block: 0.35rem; font-size: var(--text-xs); }
.alive-select__trigger:hover:not([data-disabled]), .alive-select__trigger[data-state='open'] { border-color: color-mix(in srgb, var(--c-accent) 60%, var(--c-line-strong)); background: var(--c-surface); }
.alive-select__trigger:focus-visible { outline: 2px solid var(--c-accent); outline-offset: 3px; }
.alive-select__trigger[data-disabled] { color: var(--c-ink-faint); cursor: not-allowed; opacity: 0.7; }
.alive-select__chevron { display: inline-flex; width: 0.875rem; height: 0.875rem; flex: 0 0 auto; color: var(--c-ink-muted); transition: transform var(--duration-fast) var(--ease-out); }
.alive-select__chevron svg, .alive-select__indicator svg { width: 100%; height: 100%; fill: none; stroke: currentColor; stroke-width: 1.6; stroke-linecap: round; stroke-linejoin: round; }
.alive-select__trigger[data-state='open'] .alive-select__chevron { transform: rotate(180deg); }
.alive-select__content { z-index: 50; min-width: var(--reka-select-trigger-width); max-height: min(20rem, var(--reka-select-content-available-height)); overflow: hidden; border: 1px solid var(--c-glass-border); border-radius: 0.875rem; background: var(--c-glass); box-shadow: 0 0.75rem 2rem var(--c-shadow-float); backdrop-filter: blur(18px) saturate(140%); transform-origin: var(--reka-select-content-transform-origin); animation: alive-select-open var(--duration-fast) var(--ease-out); }
.alive-select__viewport { padding: var(--space-1); }
.alive-select__item { position: relative; display: flex; min-height: 2.5rem; align-items: center; padding: 0.45rem 0.75rem 0.45rem 2.25rem; border-radius: 0.625rem; color: var(--c-ink); font-family: var(--font-ui); font-size: var(--text-sm); line-height: 1.35; cursor: pointer; user-select: none; }
.alive-select__item[data-highlighted] { background: var(--c-surface-sunken); outline: none; }
.alive-select__item[data-state='checked'] { color: var(--c-accent); }
.alive-select__item[data-disabled] { color: var(--c-ink-faint); cursor: not-allowed; }
.alive-select__indicator { position: absolute; left: 0.75rem; display: inline-flex; width: 0.9rem; height: 0.9rem; transform: rotate(-4deg); }
.alive-select__copy { display: grid; gap: 0.1rem; }
.alive-select__copy small { color: var(--c-ink-faint); font-size: var(--text-xs); }
@keyframes alive-select-open { from { opacity: 0; transform: translateY(-0.2rem) scale(0.985); } to { opacity: 1; transform: translateY(0) scale(1); } }
@media (pointer: coarse) { .alive-select__trigger, .alive-select__item { min-height: 44px; } }
@media (prefers-reduced-motion: reduce) { .alive-select__trigger, .alive-select__chevron { transition: none; } .alive-select__content { animation: none; } }
</style>
