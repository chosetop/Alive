<script setup lang="ts">
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from 'reka-ui'
import { computed } from 'vue'

/**
 * An actions menu.
 *
 * Items arrive as data rather than as slotted markup. That is the deliberate
 * choice: the slash command palette and the header's more-actions menu both need
 * the same keyboard behaviour over a list that is computed, and a slot-based API
 * would have each call site rebuilding item wiring — which is where roving
 * tabindex and typeahead get dropped.
 *
 * Reka provides arrow-key navigation, Home/End, typeahead, Escape, and focus
 * return. A `danger` item is styled apart from its neighbours, and the plan's
 * constraint that destructive and primary actions never sit adjacent is a layout
 * decision left to call sites.
 */
export interface UiMenuItem {
  id: string
  label: string
  /** Rendered muted after the label — a shortcut hint, not a second action. */
  hint?: string
  disabled?: boolean
  /** Draws a separator above this item. */
  separated?: boolean
  danger?: boolean
}

export interface UiMenuProps {
  items: UiMenuItem[]
  /** Controlled open state. Omit to let the trigger own it. */
  open?: boolean
  /** Accessible name for the menu itself. */
  label?: string
  side?: 'top' | 'right' | 'bottom' | 'left'
  align?: 'start' | 'center' | 'end'
}

const props = withDefaults(defineProps<UiMenuProps>(), {
  open: undefined,
  label: undefined,
  side: 'bottom',
  align: 'end',
})

const emit = defineEmits<{
  /** The chosen item's id. Business components switch on this, never on indices. */
  select: [string]
  'update:open': [boolean]
}>()

/** Forwards `undefined` so uncontrolled use keeps working. See UiDialog. */
const openModel = computed({
  get: () => props.open,
  set: (value: boolean | undefined) => emit('update:open', value ?? false),
})
</script>

<template>
  <DropdownMenuRoot v-model:open="openModel">
    <DropdownMenuTrigger v-if="$slots.trigger" as-child>
      <slot name="trigger" />
    </DropdownMenuTrigger>

    <DropdownMenuPortal>
      <DropdownMenuContent
        class="ui-menu__content"
        :side="side"
        :align="align"
        :side-offset="6"
        :aria-label="label"
      >
        <template v-for="item in items" :key="item.id">
          <DropdownMenuSeparator v-if="item.separated" class="ui-menu__separator" />
          <DropdownMenuItem
            class="ui-menu__item"
            :disabled="item.disabled"
            :data-danger="item.danger ? 'true' : undefined"
            @select="emit('select', item.id)"
          >
            <span class="ui-menu__label">{{ item.label }}</span>
            <span v-if="item.hint" class="ui-menu__hint">{{ item.hint }}</span>
          </DropdownMenuItem>
        </template>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
