<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const props = withDefaults(defineProps<{
  label: string
  align?: 'left' | 'right'
}>(), {
  align: 'left',
})

const root = ref<HTMLElement | null>(null)
const open = ref(false)

function openPopover(): void {
  open.value = true
}

function closePopover(): void {
  open.value = false
}

function closeOnOutsidePointerDown(event: PointerEvent): void {
  const target = event.target
  if (open.value && target instanceof Node && !root.value?.contains(target)) closePopover()
}

function closeOnEscape(event: KeyboardEvent): void {
  if (event.key === 'Escape' && open.value) {
    closePopover()
    root.value?.querySelector<HTMLButtonElement>('.trigger')?.focus()
  }
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
  <div
    ref="root"
    class="popover-toggle"
    :class="`popover-toggle--${props.align}`"
    @mouseenter="openPopover"
    @mouseleave="closePopover"
  >
    <button
      type="button"
      class="trigger"
      :aria-label="props.label"
      :aria-expanded="open"
      aria-haspopup="menu"
      @click.prevent
      @focusin="openPopover"
    >
      <span class="trigger-content"><slot name="trigger" /></span>
    </button>

    <div v-if="open" class="menu" role="menu">
      <slot name="menu" :close="closePopover" />
    </div>
  </div>
</template>

<style scoped>
.popover-toggle {
  position: relative;
  min-width: max-content;
}

.popover-toggle::after {
  position: absolute;
  top: 100%;
  width: 100%;
  height: var(--space-2);
  content: '';
}

.popover-toggle--left::after {
  left: 0;
}

.popover-toggle--right::after {
  right: 0;
}

.trigger {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  min-height: 2.5rem;
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--c-line-strong);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 88%, var(--c-accent));
  color: var(--c-ink-muted);
  font-family: var(--font-ui);
  font-size: var(--text-sm);
  cursor: pointer;
  transition: border-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out), transform var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
}

.trigger-content::before {
  display: inline-block;
  width: 0.5rem;
  height: 0.5rem;
  margin-right: var(--space-2);
  border-radius: 50%;
  background: var(--c-accent);
  box-shadow: 0 0 0 0.2rem color-mix(in srgb, var(--c-accent) 16%, transparent);
  content: '';
  vertical-align: 0.05em;
}

.trigger-content::after {
  display: inline-block;
  width: 0.35rem;
  height: 0.35rem;
  margin-left: var(--space-2);
  border-right: 1px solid currentColor;
  border-bottom: 1px solid currentColor;
  content: '';
  transform: rotate(45deg) translateY(-0.1rem);
  transition: transform var(--duration-fast) var(--ease-out);
}

.trigger:hover,
.trigger:focus-visible,
.popover-toggle:hover .trigger {
  border-color: var(--c-accent);
  color: var(--c-accent);
  outline: none;
  transform: translateY(-1px);
}

.popover-toggle:hover .trigger-content::after,
.trigger:focus-visible .trigger-content::after {
  transform: rotate(225deg) translate(-0.05rem, -0.05rem);
}

.menu {
  position: absolute;
  top: calc(100% + var(--space-2));
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
  animation: popover-in var(--duration-fast) var(--ease-out);
}

.popover-toggle--left .menu {
  left: 0;
}

.popover-toggle--right .menu {
  right: 0;
}

.menu :deep(button) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  width: 100%;
  min-height: 2.5rem;
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

.menu :deep(button:hover),
.menu :deep(button[aria-checked='true']) {
  background: var(--c-surface-sunken);
  color: var(--c-accent);
}

.menu :deep(button[aria-checked='true']::after) {
  content: '✓';
  font-size: var(--text-xs);
}

.menu :deep(button:active) {
  transform: scale(0.98);
}

@keyframes popover-in {
  from { opacity: 0; transform: translateY(-0.25rem) scale(0.98); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

@media (prefers-reduced-motion: reduce) {
  .trigger,
  .trigger-content::after,
  .menu,
  .menu :deep(button) {
    animation: none;
    transition: none;
  }
}
</style>
