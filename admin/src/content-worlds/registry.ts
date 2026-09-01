import type { WorldKey } from '../types/api'

export type AdminEditorKind = 'long-form' | 'saying'
export type WorldMaterial = 'manuscript' | 'note' | 'viewfinder'

export interface AdminWorldDefinition {
  key: WorldKey
  label: string
  createLabel: string
  editorRouteName: string | null
  publishPolicy: 'journal' | 'saying' | 'video'
  categoryEnabled: boolean
  mediaCapability: 'markdown-images' | 'none' | 'primary-video'
  editorKind: AdminEditorKind
  material: WorldMaterial
  description: string
  directoryNoun: string
  emptyCopy: string
  searchPlaceholder: string
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
    editorKind: 'long-form',
    material: 'manuscript',
    description: '长文、图片与时间留下的痕迹',
    directoryNoun: '日志',
    emptyCopy: '还没有日志。写下第一篇。',
    searchPlaceholder: '搜索日志',
  },
  {
    key: 'saying',
    label: '片语',
    createLabel: '写片语',
    editorRouteName: 'saying-editor-new',
    publishPolicy: 'saying',
    categoryEnabled: true,
    mediaCapability: 'none',
    editorKind: 'saying',
    material: 'note',
    description: '没有时间轴的短句、片语与轻量记录。',
    directoryNoun: '片语',
    emptyCopy: '还没有片语。先记下一句话。',
    searchPlaceholder: '搜索片语',
  },
  {
    key: 'video',
    label: '影像',
    createLabel: '发影像',
    editorRouteName: 'video-editor-new',
    publishPolicy: 'video',
    categoryEnabled: true,
    mediaCapability: 'primary-video',
    editorKind: 'long-form',
    material: 'viewfinder',
    description: '主视频、封面与说明。',
    directoryNoun: '影像',
    emptyCopy: '还没有影像。选择一段视频开始。',
    searchPlaceholder: '搜索影像',
  },
] as const

export function resolveAdminWorld(key: string | null | undefined): AdminWorldDefinition | null {
  if (typeof key !== 'string' || key === '') return null
  return ADMIN_WORLD_REGISTRY.find((world) => world.key === key) ?? null
}

export function listCreatableAdminWorlds(): AdminWorldDefinition[] {
  return ADMIN_WORLD_REGISTRY.filter((world) => world.editorRouteName !== null)
}
