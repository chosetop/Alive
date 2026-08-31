export type WorldKey = 'journal' | 'saying' | 'video'

export type WorldViewMode = 'stream' | 'wall' | 'focus'

export type PublicWorldSetting = {
  world: WorldKey
  nav_label: string
  sort_order: number
  default_view: WorldViewMode | ''
}
