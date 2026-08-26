import { describe, expect, it } from 'vitest'

import { markdownToText, renderMarkdown } from './index'

/**
 * These tests pin the behaviour the frontend already shipped, because this
 * package was extracted from it rather than written fresh. Anything asserted
 * here is a promise to two consumers at once: the Nuxt article page and the
 * admin preview must render the same source identically, or the preview stops
 * being a preview.
 */
describe('renderMarkdown', () => {
  it('returns empty string for blank input', () => {
    expect(renderMarkdown('')).toBe('')
    expect(renderMarkdown('   \n  ')).toBe('')
  })

  describe('raw HTML is not rendered', () => {
    // The single security decision in this module. The call sites use v-html,
    // which is only safe while the renderer cannot emit caller-supplied markup.
    it('escapes a script tag rather than emitting it', () => {
      const html = renderMarkdown('<script>alert(1)</script>')

      expect(html).not.toContain('<script>')
      expect(html).toContain('&lt;script&gt;')
    })

    it('escapes an inline event handler', () => {
      const html = renderMarkdown('<img src=x onerror="alert(1)">')

      expect(html).not.toContain('onerror="alert(1)"')
      expect(html).toContain('&lt;img')
    })
  })

  describe('heading demotion', () => {
    // The entry title is the page h1, so content headings start at h2 or the
    // document outline has two roots.
    it('demotes h1 to h2', () => {
      expect(renderMarkdown('# Heading')).toContain('<h2>Heading</h2>')
    })

    it('demotes h5 to h6', () => {
      expect(renderMarkdown('##### Heading')).toContain('<h6>Heading</h6>')
    })

    it('leaves h6 alone, having nowhere to go', () => {
      expect(renderMarkdown('###### Heading')).toContain('<h6>Heading</h6>')
    })
  })

  it('wraps tables so a wide one scrolls inside the measure', () => {
    const html = renderMarkdown('| a | b |\n|---|---|\n| 1 | 2 |')

    expect(html).toContain('<div class="table-scroll"><table>')
    expect(html).toContain('</table></div>')
  })

  describe('link attributes', () => {
    it('opens an external link in a new tab and defends the opener', () => {
      const html = renderMarkdown('[x](https://example.com)')

      expect(html).toContain('target="_blank"')
      expect(html).toContain('rel="noopener noreferrer"')
    })

    it('leaves a relative link untouched', () => {
      const html = renderMarkdown('[x](/entries/one)')

      expect(html).not.toContain('target="_blank"')
    })
  })

  it('gives images lazy loading and async decoding', () => {
    const html = renderMarkdown('![alt](/img.png)')

    expect(html).toContain('loading="lazy"')
    expect(html).toContain('decoding="async"')
  })

  describe('configured markdown-it options', () => {
    it('turns a newline inside a paragraph into a break', () => {
      expect(renderMarkdown('one\ntwo')).toContain('<br>')
    })

    it('linkifies a bare URL', () => {
      expect(renderMarkdown('see https://example.com')).toContain('<a href="https://example.com"')
    })

    it('applies typographic quotes', () => {
      // typographer: true. A regression here would show as straight quotes in
      // published prose.
      expect(renderMarkdown('"quoted"')).toContain('“quoted”')
    })

    it('renders GFM strikethrough', () => {
      expect(renderMarkdown('~~gone~~')).toContain('<s>gone</s>')
    })
  })
})

describe('markdownToText', () => {
  it('strips syntax that would show as literal characters in a snippet', () => {
    const text = markdownToText('## Heading\n\n**bold** and `code` and [link](https://x.com)')

    expect(text).toBe('Heading bold and code and link')
  })

  it('drops fenced code blocks entirely', () => {
    expect(markdownToText('before\n\n```js\nconst x = 1\n```\n\nafter')).toBe('before after')
  })

  it('drops images but keeps link text', () => {
    // Images are handled before links, since an image is a link with a leading !.
    expect(markdownToText('![pic](/a.png) and [text](/b)')).toBe('and text')
  })

  it('strips blockquote and list markers', () => {
    expect(markdownToText('> quoted\n\n- one\n- two')).toBe('quoted one two')
  })

  it('returns short text unchanged, with no ellipsis', () => {
    expect(markdownToText('short')).toBe('short')
  })

  it('truncates at the limit and appends an ellipsis', () => {
    const text = markdownToText('a'.repeat(200))

    expect(text).toHaveLength(161)
    expect(text.endsWith('…')).toBe(true)
  })

  it('honours an explicit limit', () => {
    expect(markdownToText('abcdefghij', 4)).toBe('abcd…')
  })

  it('trims trailing punctuation before the ellipsis', () => {
    // Otherwise "。…" reads as a typo rather than a truncation.
    expect(markdownToText('文字文字。文字', 5)).toBe('文字文字…')
  })
})
