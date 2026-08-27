export interface LinkShortcutEvent {
  key: string
  metaKey?: boolean
  ctrlKey?: boolean
}

export function isLinkShortcut(event: LinkShortcutEvent): boolean {
  return event.key.toLocaleLowerCase() === 'k' && Boolean(event.metaKey || event.ctrlKey)
}
