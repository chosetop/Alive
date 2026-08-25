/**
 * Theme selection.
 *
 * ## Why a cookie and not localStorage
 *
 * The server renders the page before the browser runs any JavaScript. It can
 * read a cookie sent with the request; it cannot read `localStorage`. Storing
 * the choice in `localStorage` means the server renders the default theme and
 * the browser corrects it after hydration, which is a visible flash of the wrong
 * colours on every single load. A cookie is sent with the document request, so
 * the first byte of HTML already carries the right theme.
 *
 * ## No stored default
 *
 * The cookie is unset until the reader chooses. While unset, no `data-theme`
 * attribute is written and the palette in tokens.css follows the operating
 * system through `light-dark()`. Writing a default on first visit would override
 * a reader who had already told their OS they prefer dark.
 */

export const THEMES = ['ink', 'lamp'] as const

export type ThemeName = (typeof THEMES)[number]

function isThemeName(value: unknown): value is ThemeName {
  return typeof value === 'string' && (THEMES as readonly string[]).includes(value)
}

export function useTheme() {
  const cookie = useCookie<string | null>('theme', {
    // A year: this is a preference, not a session.
    maxAge: 60 * 60 * 24 * 365,
    path: '/',
    sameSite: 'lax',
  })

  /**
   * Falls back to `ink` for display purposes only. The distinction matters:
   * `theme.value` answers "which label to show", while the attribute below is
   * absent when unset so the OS preference still wins.
   */
  const theme = computed<ThemeName>(() => (isThemeName(cookie.value) ? cookie.value : 'ink'))

  function setTheme(next: ThemeName): void {
    cookie.value = next
  }

  /**
   * Applies the attribute to `<html>`.
   *
   * `useHead` rather than touching the DOM directly, so the attribute is part of
   * the server-rendered markup instead of being added afterwards.
   */
  function applyTheme(): void {
    useHead({
      htmlAttrs: {
        // Undefined omits the attribute, which is what lets the OS decide.
        'data-theme': computed(() => (isThemeName(cookie.value) ? cookie.value : undefined)),
      },
    })
  }

  return { theme, setTheme, applyTheme }
}
