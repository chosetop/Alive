<script setup lang="ts">
import { defaultValueCtx, Editor, rootCtx } from '@milkdown/kit/core'
import { listener, listenerCtx } from '@milkdown/kit/plugin/listener'
import { commonmark } from '@milkdown/kit/preset/commonmark'
import { gfm } from '@milkdown/kit/preset/gfm'
import { history } from '@milkdown/kit/plugin/history'
import { Milkdown, useEditor } from '@milkdown/vue'
import { nord } from '@milkdown/theme-nord'

// Both stylesheets are required, not optional polish. ProseMirror's own CSS
// carries editing behaviour that is visual — selection, gap cursor, placeholder
// positioning — and without the theme the editor renders as unstyled blocks.
// They are imported here rather than in style.css so the editor's dependencies
// travel with the editor and load only in this lazy chunk.
import '@milkdown/kit/prose/view/style/prosemirror.css'
import '@milkdown/kit/prose/gapcursor/style/gapcursor.css'
import '@milkdown/theme-nord/style.css'

/**
 * The Markdown editor, wrapping Milkdown.
 *
 * WYSIWYG on the surface, Markdown underneath: what gets stored and sent is the
 * source text, so nothing here changes the backend's contract. `content_md` is
 * the single representation.
 *
 * **Not a `v-model`.** Milkdown reads its initial content once, when the editor
 * is constructed, and owns the document from then on. Feeding a prop back in on
 * every change would either rebuild the editor mid-typing — losing the cursor
 * and the undo stack — or be ignored. So the initial value is read once and
 * changes travel one way, outward, via `update`.
 *
 * The consequence for callers: this component must be mounted only once the
 * initial content is known. `EntryEditor` does that with a `v-if` on its load
 * state, which is also why it carries a `:key`.
 */

const props = defineProps<{
  /** Read once at construction. Later changes to this prop are not applied. */
  initialValue: string
}>()

const emit = defineEmits<{ update: [markdown: string] }>()

useEditor((root) =>
  Editor.make()
    .config(nord)
    .config((ctx) => {
      ctx.set(rootCtx, root)
      ctx.set(defaultValueCtx, props.initialValue)

      // markdownUpdated rather than `updated`: the parent stores Markdown, and
      // serialising here keeps the ProseMirror document from leaking outward.
      ctx.get(listenerCtx).markdownUpdated((_ctx, markdown) => {
        emit('update', markdown)
      })
    })
    // commonmark is the base; gfm adds tables, strikethrough and task lists.
    // Both are supported by the backend, which stores whatever text arrives.
    .use(commonmark)
    .use(gfm)
    .use(listener)
    // Without this, ctrl/cmd-Z inside the editor hits the browser's own undo
    // and does the wrong thing.
    .use(history),
)
</script>

<template>
  <div class="editor-shell">
    <Milkdown />
  </div>
</template>

<style scoped>
.editor-shell {
  border: 1px solid var(--c-line-strong);
  border-radius: var(--radius-sm);
  background: var(--c-paper);
}

.editor-shell:focus-within {
  border-color: var(--c-accent);
}

/* Milkdown renders into a child it owns, so these reach past scoped styles with
   :deep(). The alternative is a global stylesheet, which would then apply to any
   future editor instance whether or not that was wanted. */
.editor-shell :deep(.milkdown) {
  padding: var(--space-4);
}

.editor-shell :deep(.ProseMirror) {
  min-height: 24rem;
  outline: none;
  font-size: 0.9375rem;
  line-height: 1.7;
}

.editor-shell :deep(.ProseMirror h1),
.editor-shell :deep(.ProseMirror h2),
.editor-shell :deep(.ProseMirror h3) {
  margin: 1.5em 0 0.5em;
  line-height: 1.3;
}

.editor-shell :deep(.ProseMirror h1) {
  font-size: 1.5rem;
}

.editor-shell :deep(.ProseMirror h2) {
  font-size: 1.25rem;
}

.editor-shell :deep(.ProseMirror h3) {
  font-size: 1.0625rem;
}

.editor-shell :deep(.ProseMirror p) {
  margin: 0.75em 0;
}

.editor-shell :deep(.ProseMirror blockquote) {
  margin: 1em 0;
  padding-left: var(--space-4);
  border-left: 2px solid var(--c-line-strong);
  color: var(--c-ink-muted);
}

.editor-shell :deep(.ProseMirror code) {
  padding: 0.125em 0.25em;
  border-radius: var(--radius-sm);
  background: var(--c-surface-sunken);
  font-family: var(--font-mono);
  font-size: 0.875em;
}

.editor-shell :deep(.ProseMirror pre) {
  margin: 1em 0;
  padding: var(--space-3);
  overflow-x: auto;
  border-radius: var(--radius-sm);
  background: var(--c-surface-sunken);
}

.editor-shell :deep(.ProseMirror pre code) {
  padding: 0;
  background: transparent;
}

.editor-shell :deep(.ProseMirror ul),
.editor-shell :deep(.ProseMirror ol) {
  margin: 0.75em 0;
  padding-left: 1.5em;
}

.editor-shell :deep(.ProseMirror img) {
  max-width: 100%;
  height: auto;
}

.editor-shell :deep(.ProseMirror table) {
  width: 100%;
  margin: 1em 0;
  border-collapse: collapse;
}

.editor-shell :deep(.ProseMirror th),
.editor-shell :deep(.ProseMirror td) {
  padding: 0.375rem var(--space-3);
  border: 1px solid var(--c-line);
  text-align: left;
}

.editor-shell :deep(.ProseMirror th) {
  background: var(--c-surface-sunken);
  font-weight: 500;
}
</style>
