import { filterEditorCommands, type EditorCommand } from './editor-commands'

export function slashQuery(textBeforeCaret: string): string | null {
  const match = /(?:^|\s)\/([^\s/]*)$/.exec(textBeforeCaret)
  return match?.[1] ?? null
}

export function slashCommands(textBeforeCaret: string): EditorCommand[] {
  const query = slashQuery(textBeforeCaret)
  return query === null ? [] : filterEditorCommands(query)
}
