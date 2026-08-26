<script setup lang="ts">
import {
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
  DialogTrigger,
} from 'reka-ui'
import { computed } from 'vue'

/**
 * A modal dialog.
 *
 * Reka owns focus trapping, Escape handling, scroll locking, and focus return to
 * the trigger. None of that is reimplemented here; the wrapper exists to fix the
 * parts that are easy to get wrong per call site — chiefly that the dialog is
 * always named.
 *
 * `title` is required. Reka wires `aria-labelledby` only if something renders a
 * `DialogTitle`, and a call site that forgot one would ship a dialog announced as
 * unlabelled. Making it a prop means it cannot be forgotten.
 *
 * Open state is optional-controlled: pass nothing and the trigger drives it, or
 * pass `open` and listen to `update:open` to drive it from a store.
 */
export interface UiDialogProps {
  /** The accessible name, rendered as the visible heading. */
  title: string
  /** Controlled open state. Omit to let the trigger own it. */
  open?: boolean
  /** Hide the visible heading while keeping the accessible name. */
  hideTitle?: boolean
  /**
   * An optional description, announced after the name.
   *
   * Worth passing for a destructive confirmation, where what the action will do
   * belongs in the accessible description rather than only in body prose.
   */
  description?: string
}

const props = withDefaults(defineProps<UiDialogProps>(), {
  open: undefined,
  hideTitle: false,
  description: undefined,
})

const emit = defineEmits<{ 'update:open': [boolean] }>()

/**
 * Reka distinguishes `undefined` (uncontrolled) from a boolean (controlled), so
 * this must forward `undefined` rather than coercing to false — coercing would
 * pin every dialog shut.
 */
const openModel = computed({
  get: () => props.open,
  set: (value: boolean | undefined) => emit('update:open', value ?? false),
})
</script>

<template>
  <DialogRoot v-model:open="openModel">
    <DialogTrigger v-if="$slots.trigger" as-child>
      <slot name="trigger" />
    </DialogTrigger>

    <DialogPortal>
      <DialogOverlay class="ui-dialog__overlay" />
      <!--
        aria-describedby="undefined" when no description is given. Reka sets the
        attribute unconditionally to a generated id, so without this every dialog
        ships an aria-describedby pointing at an element that does not exist --
        verified in the DOM, not inferred. "undefined" is Reka's own sanctioned
        opt-out, not a stringified mistake.
      -->
      <DialogContent
        class="ui-dialog__content"
        :aria-describedby="description ? undefined : 'undefined'"
      >
        <DialogTitle :class="hideTitle ? 'ui-visually-hidden' : 'ui-dialog__title'">
          {{ title }}
        </DialogTitle>

        <DialogDescription v-if="description" class="ui-dialog__description">
          {{ description }}
        </DialogDescription>

        <div class="ui-dialog__body">
          <slot />
        </div>

        <footer v-if="$slots.actions" class="ui-dialog__actions">
          <slot name="actions" />
        </footer>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
