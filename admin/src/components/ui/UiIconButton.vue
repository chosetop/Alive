<script setup lang="ts">
/**
 * A button whose only content is a glyph.
 *
 * `label` is required, not optional. An icon-only control with no accessible name
 * is announced as "button" and is unusable by anyone not looking at it — and the
 * directory collapse and drawer controls are icon-only by design.
 *
 * The glyph itself is hidden from the accessibility tree: the label already names
 * the control, so exposing the SVG would announce it twice.
 */
export interface UiIconButtonProps {
  /** The accessible name. Required — see above. */
  label: string
  variant?: 'secondary' | 'quiet' | 'danger'
  disabled?: boolean
  /** Set when the button toggles something, so state is announced, not implied. */
  pressed?: boolean
}

const props = withDefaults(defineProps<UiIconButtonProps>(), {
  variant: 'quiet',
  disabled: false,
  pressed: undefined,
})

const emit = defineEmits<{ click: [MouseEvent] }>()

function handleClick(event: MouseEvent): void {
  if (props.disabled) return
  emit('click', event)
}
</script>

<template>
  <button
    class="ui-icon-button"
    type="button"
    :data-variant="variant"
    :disabled="disabled"
    :aria-label="label"
    :aria-pressed="pressed"
    @click="handleClick"
  >
    <span class="ui-icon-button__glyph" data-icon aria-hidden="true">
      <slot />
    </span>
  </button>
</template>
