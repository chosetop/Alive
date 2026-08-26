/**
 * The preview renderer, re-exported from the shared package.
 *
 * There is deliberately no second Markdown configuration here. The admin edits
 * with Milkdown, which parses through ProseMirror, but it *previews* through the
 * same markdown-it instance the published page uses. That is what makes the
 * preview trustworthy: what the author sees is produced by the renderer that will
 * produce the article, not by a lookalike that agrees until it doesn't.
 *
 * Adding an option or a renderer rule in this file would silently break that
 * guarantee, since nothing would fail — the two surfaces would simply drift.
 * Extensions belong in `@alive/markdown`, where both consumers get them.
 */
export { markdownToText, renderMarkdown } from '@alive/markdown'
