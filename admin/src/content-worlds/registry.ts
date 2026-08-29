import type { WorldKey } from '../types/api'

export interface AdminWorldDefinition {
  key: WorldKey
  label: string
  createLabel: string
  editorRouteName: string | null
  publishPolicy: 'journal' | 'saying' | 'video'
  categoryEnabled: boolean
  mediaCapability: 'markdown-images' | 'none' | 'primary-video'
}

export const ADMIN_WORLD_REGISTRY: readonly AdminWorldDefinition[] = [
  {
    key: 'journal',
    label: '日志',
    createLabel: '写日志',
    editorRouteName: 'entry-new-world',
    publishPolicy: 'journal',
    categoryEnabled: true,
    mediaCapability: 'markdown-images',
  },
  {
    key: 'saying',
    label: '片语',
    createLabel: '写片语',
    editorRouteName: null,
    publishPolicy: 'saying',
    categoryEnabled: true,
    mediaCapability: 'none',
  },
  {
    key: 'video',
    label: '影像',
    createLabel: '发影像',
    editorRouteName: null,
    publishPolicy: 'video',
    categoryEnabled: true,
    mediaCapability: 'primary-video',
  },
] as const

export function resolveAdminWorld(key: string | null | undefined): AdminWorldDefinition | null {
  if (typeof key !== 'string' || key === '') return null
  return ADMIN_WORLD_REGISTRY.find((world) => world.key === key) ?? null
}

export function listCreatableAdminWorlds(): AdminWorldDefinition[] {
  return ADMIN_WORLD_REGISTRY.filter((world) => world.editorRouteName !== null)
}
