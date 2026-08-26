<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'

/**
 * The live region that announces transient results.
 *
 * Deliberately not built on Reka's Toast: that component owns timing, swipe
 * gestures, and pause-on-hover, none of which this workspace needs. What it does
 * need is that "已发布" reaches a screen reader — the plan requires save and
 * publish state to be perceivable as text, never by colour or animation alone.
 *
 * The API is imperative (`publish`) rather than a `toasts` prop. An announcement
 * is an event, not state: exposing it as a prop would push id generation and
 * expiry timers onto every consumer, and the publish panel, the save coordinator,
 * and the directory would each grow their own copy of that bookkeeping. One owner
 * here, called from anywhere.
 *
 * `role="status"` with `aria-live="polite"`: a publish confirmation must not
 * interrupt what is being read mid-sentence. Failures use the same politeness
 * rather than `assertive`, because a failed save also stays visible in the header
 * — the announcement is a courtesy, not the only signal.
 *
 * The region renders even when empty. A live region inserted into the DOM at the
 * same moment its text appears is frequently not announced at all; it has to be
 * present and observed beforehand.
 */
export interface UiToast {
  id: string
  message: string
  tone?: 'neutral' | 'danger'
}

const props = withDefaults(defineProps<{ duration?: number }>(), {
  /**
   * Long enough to read a short Chinese phrase without hurrying, short enough not
   * to linger over the canvas.
   */
  duration: 4000,
})

const toasts = ref<UiToast[]>([])
const timers = new Set<ReturnType<typeof setTimeout>>()
let sequence = 0

/** Announces a message. Returns its id so a caller can dismiss it early. */
function publish(message: string, tone: UiToast['tone'] = 'neutral'): string {
  const id = `toast-${++sequence}`
  toasts.value = [...toasts.value, { id, message, tone }]

  const timer = setTimeout(() => {
    dismiss(id)
    timers.delete(timer)
  }, props.duration)
  timers.add(timer)

  return id
}

function dismiss(id: string): void {
  toasts.value = toasts.value.filter((toast) => toast.id !== id)
}

// Timers outlive the component otherwise, and a pending one would write to a
// disposed ref after a route change away from the workspace.
onBeforeUnmount(() => {
  for (const timer of timers) clearTimeout(timer)
  timers.clear()
})

defineExpose({ publish, dismiss })
</script>

<template>
  <div
    class="ui-toast-region"
    role="status"
    aria-live="polite"
    aria-atomic="false"
    data-testid="toast-region"
  >
    <p v-for="toast in toasts" :key="toast.id" class="ui-toast" :data-tone="toast.tone">
      {{ toast.message }}
    </p>
  </div>
</template>
