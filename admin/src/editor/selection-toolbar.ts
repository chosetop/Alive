export interface SelectionToolbarAction {
  id: 'bold' | 'emphasis' | 'strike' | 'inline-code'
  label: string
}

export const SELECTION_TOOLBAR_ACTIONS: readonly SelectionToolbarAction[] = [
  { id: 'bold', label: '粗体' },
  { id: 'emphasis', label: '斜体' },
  { id: 'strike', label: '删除线' },
  { id: 'inline-code', label: '行内代码' },
]

export function shouldShowSelectionToolbar(from: number, to: number): boolean {
  return to > from
}
