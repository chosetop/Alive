import type { Category } from '~/types'
import type { WorldKey } from '~/types'

/**
 * The site's categories, fetched once per render and shared.
 *
 * This exists because `useAsyncData` keys the cache by its first argument, and
 * two call sites using the same key with different options is a real conflict:
 * Nuxt warns, and which set of options takes effect depends on which component
 * mounted first. Both the layout's footer nav and the category page need this
 * list, so the call lives in one place and they both go through it.
 *
 * Failure is deliberately non-fatal. Categories are navigation; if the request
 * fails the footer is empty, but the entry the reader came for still renders.
 * `default` supplies an empty array so no caller has to handle `null`.
 */
export function useSiteCategories(world: WorldKey = 'journal') {
  const { list } = useCategoriesApi()

  return useAsyncData<Category[]>(`site-categories-${world}`, () => list({ world }), {
    default: () => [],
  })
}
