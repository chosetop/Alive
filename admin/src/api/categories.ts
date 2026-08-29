import type {
  Category,
  CategoryCreateRequest,
  CategoryUpdateRequest,
  CategoryWithCount,
  WorldKey,
} from '../types/api'
import { request } from './client'

/**
 * Category endpoints.
 *
 * Note the path asymmetry: writes are on `/categories/:id`, reads on
 * `/admin/categories`. That is the server's shape, not a mistake here — the
 * public list and the admin list return different fields (`entry_count` versus
 * timestamps), so they are two endpoints, while a write has only one form and
 * needs no second path.
 */

/**
 * GET /api/v1/categories
 *
 * Public and **not paginated**, so the envelope has no `meta`. This must use
 * `request`, not `requestPaginated` — the latter throws `INTERNAL` when `meta`
 * is missing, which is deliberate: a silent fallback would hide a real contract
 * mismatch until some later screen showed the wrong page count.
 *
 * Returns `entry_count` but no timestamps. The admin list uses
 * `listCategoriesAdmin` instead; this one exists for the counts.
 */
export function listCategoriesPublic(query: { world: WorldKey }): Promise<CategoryWithCount[]> {
  return request<CategoryWithCount[]>('/categories', { query })
}

/**
 * GET /api/v1/admin/categories
 *
 * Also not paginated, for the same reason. Returns timestamps, no counts.
 */
export function listCategoriesAdmin(query: { world: WorldKey }): Promise<Category[]> {
  return request<Category[]>('/admin/categories', { query })
}

/** GET /api/v1/admin/categories/:id */
export function getCategory(id: number): Promise<Category> {
  return request<Category>(`/admin/categories/${id}`)
}

/**
 * POST /api/v1/categories
 *
 * 409 with `fields.slug` when the slug is taken — not 400, because the request
 * is well-formed and a different slug would succeed.
 */
export function createCategory(body: CategoryCreateRequest): Promise<Category> {
  return request<Category>('/categories', { method: 'POST', body })
}

/**
 * PATCH /api/v1/categories/:id
 *
 * The caller is responsible for sending only the fields it means to change,
 * and for using `""` / `0` rather than `null` to clear one. See docs/api.md 1.7
 * and `buildCategoryPatch`, which is the only place that logic should live.
 *
 * An empty patch is a 400 from the server. `buildCategoryPatch` returning an
 * empty object is therefore a signal to skip the request, not to send it.
 */
export function updateCategory(id: number, body: CategoryUpdateRequest): Promise<Category> {
  return request<Category>(`/categories/${id}`, { method: 'PATCH', body })
}

/**
 * DELETE /api/v1/categories/:id
 *
 * Physical delete, 204, and 404 on a second call.
 *
 * **Entries in the category are not deleted.** The foreign key is
 * `ON DELETE SET NULL`, so they become uncategorised. Worth saying plainly in
 * the confirmation dialog: "the N entries here become uncategorised" is a
 * different promise from "N entries will be deleted".
 */
export function deleteCategory(id: number): Promise<void> {
  return request<void>(`/categories/${id}`, { method: 'DELETE' })
}
