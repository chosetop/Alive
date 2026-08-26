<script setup lang="ts">
/**
 * The one button in the workspace.
 *
 * Variants change colour and weight only. Typography is identical across all
 * four, because a button that also changes size reads as a different kind of
 * control and makes a row of actions look assembled rather than designed.
 *
 * `data-variant` rather than a class: the styling hook stays stable regardless of
 * how scoped-CSS hashing renames things, and `ui.css` can target it without
 * knowing this component's internals.
 */
export interface UiButtonProps {
  variant?: 'primary' | 'secondary' | 'quiet' | 'danger'
  /** Native disabled. Also suppresses the click emit. */
  disabled?: boolean
  /**
   * An in-flight action. Announced with `aria-busy` rather than shown only as a
   * spinner, and treated as disabled so a second click cannot start a second
   * publish request.
   */
  loading?: boolean
  type?: 'button' | 'submit'
}

const props = withDefaults(defineProps<UiButtonProps>(), {
  variant: 'secondary',
  disabled: false,
  loading: false,
  // Not 'submit'. These sit beside inputs in the settings drawer, and a bare
  // button inside a form submits it.
  type: 'button',
})

const emit = defineEmits<{ click: [MouseEvent] }>()

function handleClick(event: MouseEvent): void {
  if (props.disabled || props.loading) return
  emit('click', event)
}
</script>

<template>
  <button
    class="ui-button"
    :type="type"
    :data-variant="variant"
    :disabled="disabled || loading"
    :aria-busy="loading ? 'true' : undefined"
    @click="handleClick"
  >
    <slot />
  </button>
</template>
