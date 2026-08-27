export type EditorCommandId =
  | 'heading-2'
  | 'heading-3'
  | 'bullet-list'
  | 'ordered-list'
  | 'quote'
  | 'code'
  | 'divider'

export interface EditorCommandContext {
  replaceSlashQuery(): void
  toggleBlock(type: 'heading' | 'bullet_list' | 'ordered_list' | 'blockquote' | 'code_block' | 'horizontal_rule', level?: number): void
}

export interface EditorCommand {
  id: EditorCommandId
  label: string
  keywords: string[]
  run(context: EditorCommandContext): boolean
}

function command(
  id: EditorCommandId,
  label: string,
  keywords: string[],
  runBlock: (context: EditorCommandContext) => void,
): EditorCommand {
  return {
    id,
    label,
    keywords,
    run(context) {
      context.replaceSlashQuery()
      runBlock(context)
      return true
    },
  }
}

export const EDITOR_COMMANDS: readonly EditorCommand[] = [
  command('heading-2', '二级标题', ['heading', 'h2', '标题'], (context) => context.toggleBlock('heading', 2)),
  command('heading-3', '三级标题', ['heading', 'h3', '标题'], (context) => context.toggleBlock('heading', 3)),
  command('bullet-list', '无序列表', ['bullet', 'list', '列表'], (context) => context.toggleBlock('bullet_list')),
  command('ordered-list', '有序列表', ['ordered', 'numbered', 'list', '列表'], (context) => context.toggleBlock('ordered_list')),
  command('quote', '引用', ['quote', 'blockquote'], (context) => context.toggleBlock('blockquote')),
  command('code', '代码块', ['code', 'preformatted'], (context) => context.toggleBlock('code_block')),
  command('divider', '分隔线', ['divider', 'rule', 'horizontal'], (context) => context.toggleBlock('horizontal_rule')),
]

export function filterEditorCommands(query: string): EditorCommand[] {
  const normalized = query.trim().toLocaleLowerCase()
  if (normalized === '') return [...EDITOR_COMMANDS]
  return EDITOR_COMMANDS.filter((command) =>
    [command.label, ...command.keywords].some((value) => value.toLocaleLowerCase().includes(normalized)),
  )
}
