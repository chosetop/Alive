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

const PUBLIC_WORLDS = [JOURNAL_WORLD, SAYING_WORLD] as const

export function listPublicWorlds(): readonly PublicWorldDefinition[] {
  return PUBLIC_WORLDS
}

export function resolvePublicWorld(key: WorldKey | string): PublicWorldDefinition | null {
  return key === 'journal' ? JOURNAL_WORLD : key === 'saying' ? SAYING_WORLD : null
}
