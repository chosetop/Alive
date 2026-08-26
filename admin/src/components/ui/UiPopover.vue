<script setup lang="ts">
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'
import { computed } from 'vue'

/**
 * A non-modal overlay anchored to its trigger.
 *
 * Popover rather than Dialog where the surrounding page must stay usable — the
 * link editor is the case that matters, since the editor selection behind it has
 * to remain visible. Reka handles outside-click dismissal, Escape, and focus
 * return; positioning comes from Floating UI underneath.
 *
 * No `title` prop, unlike UiDialog: a popover is not announced as a named region,
 * and requiring a heading would push call sites into rendering one they do not
 * want. Call sites that need a name pass `label`.
 */
export interface UiPopoverProps {
  /** Controlled open state. Omit to let the trigger own it. */
  open?: boolean
  /** Accessible name for the content region, when it needs one. */
  label?: string
  side?: 'top' | 'right' | 'bottom' | 'left'
  align?: 'start' | 'center' | 'end'
}

const props = withDefaults(defineProps<UiPopoverProps>(), {
  open: undefined,
  label: undefined,
  side: 'bottom',
  align: 'start',
})

const emit = defineEmits<{ 'update:open': [boolean] }>()

/** Forwards `undefined` so uncontrolled use keeps working. See UiDialog. */
const openModel = computed({
  get: () => props.open,
  set: (value: boolean | undefined) => emit('update:open', value ?? false),
})
</script>

<template>
  <PopoverRoot v-model:open="openModel">
    <PopoverTrigger v-if="$slots.trigger" as-child>
      <slot name="trigger" />
    </PopoverTrigger>

    <PopoverPortal>
      <PopoverContent
        class="ui-popover__content"
        :side="side"
        :align="align"
        :side-offset="6"
        :aria-label="label"
      >
        <slot />
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
