import type {
  AdminCategory,
  Category,
  CategoryCreateBody,
  CategoryListQuery,
  CategoryUpdateBody,
} from '~/types'
import { useApi } from './useApi'

/**
 * Category endpoints. Source of truth: docs/api.md section 5.
 *
 * Writes address categories by id, because the slug is one of the things being
 * edited. The public list is the only place `entry_count` appears.
 */
export function useCategoriesApi() {
  const api = useApi()

  return {
    /**
     * Public. **Unpaginated**, so the response carries no `meta` block — do not
     * read it as a page. Ordered by `sort_order`, then id.
     *
     * Empty categories are included with `entry_count: 0` rather than omitted.
     */
    async list(query: CategoryListQuery = { world: 'journal' }): Promise<Category[]> {
      const response = await api.getList<Category>('/categories', query)
      return response.data
    },

    /** Requires a session. 201. Response has timestamps but no `entry_count`. */
    create(body: CategoryCreateBody): Promise<AdminCategory> {
      return api.post<AdminCategory>('/categories', body)
    },

    /**
     * Requires a session. Field conventions as in docs/api.md 1.7: omitted or
     * `null` leaves a field alone, an empty value clears it. Body passed through
     * unchanged.
     */
    update(id: number, body: CategoryUpdateBody): Promise<AdminCategory> {
      return api.patch<AdminCategory>(`/categories/${id}`, body)
    },

    /**
     * Requires a session. **Hard delete**, 204. A second call 404s.
     *
     * Entries in the category are not deleted and the request is not refused —
     * they become uncategorised (`ON DELETE SET NULL`). This is irreversible and
     * leaves no record of which entries were affected, so one 204 can silently
     * change many rows.
     */
    remove(id: number): Promise<void> {
      return api.requestNoContent('DELETE', `/categories/${id}`)
    },

    /** Requires a session. All categories with timestamps, no counts. Unpaginated. */
    async adminList(): Promise<AdminCategory[]> {
      const response = await api.getList<AdminCategory>('/admin/categories')
      return response.data
    },

    /**
     * Requires a session. Single category by id.
     *
     * A path segment that is not a positive integer (`abc`, `0`, `-1`) is a
     * **400**, not a 404: the request never named a category, so there is no
     * missing resource to report.
     */
    adminGet(id: number): Promise<AdminCategory> {
      return api.get<AdminCategory>(`/admin/categories/${id}`)
    },
  }
}
