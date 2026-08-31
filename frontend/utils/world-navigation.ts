import type { PublicWorldSetting } from '~/types'

import { resolvePublicWorld } from '~/content-worlds/registry'

export type WorldNavigationItem = {
  key: PublicWorldSetting['world']
  label: string
  rootPath: string
}

export function buildWorldNavigation(settings: PublicWorldSetting[]): WorldNavigationItem[] {
  return settings
    .toSorted((left, right) => left.sort_order - right.sort_order)
    .map((setting) => {
      const world = resolvePublicWorld(setting.world)
      if (!world) return null

      return {
        key: setting.world,
        label: setting.nav_label || world.label,
        rootPath: world.rootPath,
      }
    })
    .filter((item): item is WorldNavigationItem => item !== null)
}
