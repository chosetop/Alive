import type { WorldKey } from '~/types'

export type PublicWorldDefinition = {
  key: WorldKey
  label: string
  rootPath: string
  entryPath: (slug: string) => string
  categoryPath: (slug: string) => string
  timePolicy: 'timeline' | 'hidden' | 'metadata'
  seoType: 'Article' | 'SocialMediaPosting' | 'VideoObject'
}

const JOURNAL_WORLD: PublicWorldDefinition = {
  key: 'journal',
  label: '日志',
  rootPath: '/journal',
  entryPath: (slug) => `/journal/${slug}`,
  categoryPath: (slug) => `/journal/categories/${slug}`,
  timePolicy: 'timeline',
  seoType: 'Article',
}

const SAYING_WORLD: PublicWorldDefinition = {
  key: 'saying',
  label: '片语',
  rootPath: '/sayings',
  entryPath: (slug) => `/sayings/${slug}`,
  categoryPath: (slug) => `/sayings/categories/${slug}`,
  timePolicy: 'hidden',
  seoType: 'SocialMediaPosting',
}

const VIDEO_WORLD: PublicWorldDefinition = {
  key: 'video',
  label: '影像',
  rootPath: '/videos',
  entryPath: (slug) => `/videos/${slug}`,
  categoryPath: (slug) => `/videos/categories/${slug}`,
  timePolicy: 'metadata',
  seoType: 'VideoObject',
}

const PUBLIC_WORLDS = [JOURNAL_WORLD, SAYING_WORLD, VIDEO_WORLD] as const

export function listPublicWorlds(): readonly PublicWorldDefinition[] {
  return PUBLIC_WORLDS
}

export function resolvePublicWorld(key: WorldKey | string): PublicWorldDefinition | null {
  return key === 'journal' ? JOURNAL_WORLD : key === 'saying' ? SAYING_WORLD : key === 'video' ? VIDEO_WORLD : null
}

/** The world root opens its first configured category when one exists. */
export function defaultWorldCategoryPath(
  key: WorldKey,
  categories: readonly { slug: string }[],
): string | null {
  const world = resolvePublicWorld(key)
  const first = categories[0]
  return world && first ? world.categoryPath(first.slug) : null
}
