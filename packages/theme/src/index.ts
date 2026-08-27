export const THEMES = [
  { name: 'ink', label: '纸墨', colorScheme: 'light' },
  { name: 'lamp', label: '灯下', colorScheme: 'dark' },
  { name: 'codex-lavender', label: 'Codex Lavender', colorScheme: 'light' },
] as const

export type ThemeName = (typeof THEMES)[number]['name']

export function isThemeName(value: unknown): value is ThemeName {
  return THEMES.some((theme) => theme.name === value)
}

export function resolveTheme(input: { visitor: unknown; siteDefault: unknown }): ThemeName {
  if (isThemeName(input.visitor)) return input.visitor
  if (isThemeName(input.siteDefault)) return input.siteDefault
  return 'ink'
}
