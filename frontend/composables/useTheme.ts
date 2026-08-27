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
 * The cookie is unset until the reader chooses. While unset, the resolved
 * theme is the site's server-backed default.
 */

import { isThemeName, resolveTheme, type ThemeName } from '@alive/theme'
import type { Ref } from 'vue'

export function resolveVisitorTheme(visitor: unknown, siteDefault: unknown): ThemeName {
  return resolveTheme({ visitor, siteDefault })
}

export function useTheme(siteDefault: Ref<string>) {
  const cookie = useCookie<string | null>('theme', {
    // A year: this is a preference, not a session.
    maxAge: 60 * 60 * 24 * 365,
    path: '/',
    sameSite: 'lax',
  })

  /** The resolved theme is always valid, including while settings are loading. */
  const visitorTheme = computed<ThemeName | null>(() =>
    isThemeName(cookie.value) ? cookie.value : null,
  )
  const theme = computed<ThemeName>(() => resolveVisitorTheme(visitorTheme.value, siteDefault.value))

  function setVisitorTheme(next: ThemeName | null): void {
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
        'data-theme': computed(() => theme.value),
      },
    })
  }

  return { theme, visitorTheme, setVisitorTheme, applyTheme }
}
