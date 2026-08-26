/**
 * Markdown rendering, re-exported from the shared package.
 *
 * The implementation moved to `@alive/markdown` because the admin preview needs
 * the identical renderer. Two configurations meant "preview" was a claim rather
 * than a fact: any divergence in options or renderer rules would show the author
 * one document and the reader another, with nothing failing to announce it.
 *
 * This file stays as the frontend's import path so call sites and Nuxt's
 * auto-import conventions keep working. Add nothing here — a rule defined only on
 * this side would reintroduce exactly the divergence the move removed.
 */
export { markdownToText, renderMarkdown } from '@alive/markdown'
