import MarkdownIt from 'markdown-it'

/**
 * Markdown to HTML, for rendering entry bodies.
 *
 * Runs during SSR so readers and crawlers receive real HTML rather than a shell
 * that fills in after hydration. `content_md` is the only stored representation
 * (docs/architecture.md decision 4); nothing here writes back.
 *
 * ## On `html: false`
 *
 * Raw HTML in Markdown source is **not** rendered. This is the single security
 * decision in this file and it is what makes the `v-html` at the call site safe:
 * markdown-it's own output is constructed from its token stream and contains no
 * caller-supplied markup, so there is no injection surface to sanitise.
 *
 * Turning this on would require a real sanitiser (DOMPurify or equivalent) in
 * front of every render. The trade is deliberate: this is a single-author site,
 * the author has no need for raw `<div>`s in prose, and "the author is trusted"
 * is a weaker guarantee than "the renderer cannot emit arbitrary HTML" -- a
 * stored XSS only needs one pasted snippet to become permanent.
 *
 * ## Why this lives in a package
 *
 * Two consumers render the same stored Markdown: the Nuxt article page and the
 * admin preview. When each owned its own markdown-it configuration, "preview"
 * was a claim rather than a fact — a differing option or renderer rule would
 * show the author one document and the reader another, and nothing would fail to
 * announce it. One module, imported by both, makes that divergence impossible to
 * introduce by omission.
 *
 * Exported as source rather than built output. There is no build step to keep in
 * sync, and both consumers already compile TypeScript.
 *
 * ## Divergence from the admin editor
 *
 * The admin *edits* with Milkdown, which parses through ProseMirror rather than
 * markdown-it. docs/architecture.md 5.3 asks for one shared Markdown config
 * across both, which is no longer literally satisfiable for the editing surface.
 * Both cover CommonMark plus GFM, so the overlap is wide, but any syntax added on
 * one side only will render there and appear as plain text here. Extensions
 * therefore have to be added to both, or to neither.
 *
 * The admin *preview* has no such caveat: it renders through this module, so it
 * matches the published page exactly.
 */

const md: MarkdownIt = new MarkdownIt({
  // See the note above. Do not flip this without adding a sanitiser.
  html: false,
  /** Newline inside a paragraph becomes `<br>`, matching what most editors imply. */
  breaks: true,
  /** Bare URLs become links. */
  linkify: true,
  /**
   * Smart quotes and dashes. The quote characters are Chinese-appropriate: the
   * default set is `“”‘’`, which is correct for Simplified Chinese as well as
   * English, so no override is needed. Left explicit so the choice is visible.
   */
  typographer: true,
})

const legacyBlockBreakMarker = '\uE000ALIVE_BLOCK_BREAK\uE001'

/**
 * Older editor content may have persisted an HTML break instead of Markdown.
 * Keep the renderer's raw-HTML guard enabled, but recover this one harmless
 * legacy representation before parsing so `<br />` is not shown as prose.
 */
function normalizeLegacyMarkdown(source: string): string {
  return source
    // Some older content wrapped an image URL in a link and escaped the
    // opening parenthesis: ![]\([url](url)). Recover the intended image before
    // markdown-it sees the malformed inline syntax.
    .replace(
      /!\[\]\s*\\?\(\[([^\]\n]+)\]\(\1\)\)/g,
      '![]($1)',
    )
    // Another editor export escaped the image punctuation and wrapped the
    // URL in angle brackets: !\[]\(<url>). Recover the standard image form.
    .replace(
      /!\\?\[\]\\?\(\<([^>\n]+)\>\)/g,
      '![]($1)',
    )
    // The old editor put an explicit break tag on its own line between
    // paragraphs. Preserve only that authored break; ordinary blank lines
    // remain ordinary Markdown paragraph boundaries.
    .replace(
      /\n[ \t]*\n[ \t]*<br\s*\/?>[ \t]*\n[ \t]*\n/gi,
      `\n\n${legacyBlockBreakMarker}\n\n`,
    )
    // Two trailing spaces are Markdown's explicit hard-break syntax.
    .replace(/<br\s*\/?>/gi, '  \n')
}

function restoreLegacyBlockBreaks(html: string): string {
  return html.replaceAll(`<p>${legacyBlockBreakMarker}</p>\n`, '<br />\n')
}

/**
 * Wrap tables so a wide one scrolls inside the measure instead of widening the
 * page. markdown-it emits no wrapper of its own, so one is added around the
 * whole token range.
 */
md.renderer.rules.table_open = () => '<div class="table-scroll"><table>'
md.renderer.rules.table_close = () => '</table></div>'

/**
 * Demote every heading by one level.
 *
 * The entry title is the page's `h1`. Content headings must therefore start at
 * `h2`, or a document that begins with `# Something` produces two `h1`s and an
 * outline with two roots, which is what screen readers and crawlers read as the
 * page structure.
 *
 * `h6` cannot be demoted further and is left alone; six levels of nesting in a
 * blog post is not a case worth handling.
 */
function demoteHeadings(tokens: { tag: string }[]): void {
  for (const token of tokens) {
    const match = /^h([1-5])$/.exec(token.tag)
    if (match) token.tag = `h${Number(match[1]) + 1}`
  }
}

/**
 * Mark external links so they open in a new tab, and defend the opener.
 *
 * `rel="noopener"` is the one that matters: without it the opened page can reach
 * back through `window.opener`. `noreferrer` also withholds the referrer, which
 * is the polite default for a personal site.
 */
function markExternalLinks(tokens: ReturnType<MarkdownIt['parse']>): void {
  for (const token of tokens) {
    if (token.type !== 'link_open') continue
    const href = token.attrGet('href')
    if (href === null || !/^https?:\/\//i.test(href)) continue
    token.attrSet('target', '_blank')
    token.attrSet('rel', 'noopener noreferrer')
  }
}

/**
 * Give images lazy loading and intrinsic-size hints.
 *
 * `loading="lazy"` keeps a long photo post from fetching everything at once.
 * `decoding="async"` stops a large image blocking paint.
 */
function annotateImages(tokens: ReturnType<MarkdownIt['parse']>): void {
  for (const token of tokens) {
    if (token.type !== 'image') continue
    token.attrSet('loading', 'lazy')
    token.attrSet('decoding', 'async')
  }
}

/** Render Markdown source to HTML. Returns `''` for empty input. */
export function renderMarkdown(source: string): string {
  if (source.trim() === '') return ''

  const tokens = md.parse(normalizeLegacyMarkdown(source), {})

  // Inline tokens carry their own children, where links and images live.
  const inlineChildren = tokens.flatMap((token) => token.children ?? [])

  demoteHeadings(tokens)
  markExternalLinks(inlineChildren)
  annotateImages(inlineChildren)

  return restoreLegacyBlockBreaks(md.renderer.render(tokens, md.options, {}))
}

/**
 * Plain text from Markdown, for meta descriptions and list excerpts.
 *
 * Not a parser: it strips the syntax that would otherwise show up as literal
 * `##` and `**` in a search result snippet. Used only where an entry has no
 * `summary` of its own, since a hand-written summary is always better than a
 * truncated body.
 */
export function markdownToText(source: string, limit = 160): string {
  const text = normalizeLegacyMarkdown(source)
    // Fenced code blocks, which never make sense in a description.
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]*)`/g, '$1')
    // Images before links: an image is a link with a leading `!`.
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/^\s*>\s?/gm, '')
    .replace(/^\s*[-*+]\s+/gm, '')
    .replace(/[*_~]/g, '')
    .replace(/\s+/g, ' ')
    .trim()

  if (text.length <= limit) return text
  // Trailing punctuation before the ellipsis reads as a typo.
  return `${text.slice(0, limit).replace(/[\s,.;:，。；：、]+$/, '')}…`
}
