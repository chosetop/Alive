import { slashFactory, SlashProvider } from '@milkdown/kit/plugin/slash'
import { commandsCtx } from '@milkdown/kit/core'
import type { Ctx } from '@milkdown/ctx'
import type { EditorView } from '@milkdown/prose/view'
import {
  wrapInBlockquoteCommand,
  wrapInBulletListCommand,
  wrapInHeadingCommand,
  wrapInOrderedListCommand,
  createCodeBlockCommand,
  insertHrCommand,
} from '@milkdown/kit/preset/commonmark'
import { createApp, computed, defineComponent, h, ref, shallowRef } from 'vue'

import { UiMenu, type UiMenuItem } from '../components/ui'
import { filterEditorCommands, type EditorCommandContext } from './editor-commands'
import { slashQuery } from './slash-menu'

export interface SlashMenuContext {
  textBeforeCaret: string
  nodeType: string
  atEnd: boolean
}

export function isSlashMenuContext(context: SlashMenuContext): boolean {
  return context.nodeType === 'paragraph' && context.atEnd && slashQuery(context.textBeforeCaret) !== null
}

export function slashQueryRange(from: number, query: string): { from: number; to: number } {
  return { from: Math.max(0, from - query.length - 1), to: from }
}

export const slashMenuPlugin = slashFactory('ALIVE_SLASH_MENU')

function commandContext(ctx: Ctx, view: EditorView, query: string): EditorCommandContext {
  return {
    replaceSlashQuery() {
      const range = slashQueryRange(view.state.selection.from, query)
      view.dispatch(view.state.tr.delete(range.from, range.to))
    },
    toggleBlock(type, level) {
      const commands = ctx.get(commandsCtx)
      if (type === 'heading') commands.call(wrapInHeadingCommand.key, level ?? 2)
      if (type === 'bullet_list') commands.call(wrapInBulletListCommand.key)
      if (type === 'ordered_list') commands.call(wrapInOrderedListCommand.key)
      if (type === 'blockquote') commands.call(wrapInBlockquoteCommand.key)
      if (type === 'code_block') commands.call(createCodeBlockCommand.key)
      if (type === 'horizontal_rule') commands.call(insertHrCommand.key)
    },
  }
}

function createSlashMenuView(ctx: Ctx, view: EditorView) {
  const content = document.createElement('div')
  content.className = 'alive-slash-menu'
  const visible = ref(false)
  const currentView = shallowRef(view)
  let provider: SlashProvider
  provider = new SlashProvider({
    content,
    debounce: 20,
    root: view.dom.parentElement ?? undefined,
    shouldShow(nextView) {
      const selection = nextView.state.selection
      const parent = selection.$from.parent
      return isSlashMenuContext({
        textBeforeCaret: provider.getContent(nextView) ?? '',
        nodeType: parent.type.name,
        atEnd: selection.empty && selection.$from.parentOffset === parent.content.size,
      })
    },
  })

  const app = createApp(defineComponent({
    setup() {
      const items = computed<UiMenuItem[]>(() => {
        const commands = filterEditorCommands(slashQuery(provider.getContent(currentView.value) ?? '') ?? '')
        return commands.map((command) => ({ id: command.id, label: command.label }))
      })
      const select = (id: string) => {
        const query = slashQuery(provider.getContent(currentView.value) ?? '')
        if (query === null) return
        const command = filterEditorCommands(query)
          .find((candidate) => candidate.id === id)
        if (!command) return
        command.run(commandContext(ctx, currentView.value, query))
        provider.hide()
      }
      return () => h(UiMenu, { items: items.value, open: visible.value, label: '插入块', onSelect: select })
    },
  }))
  app.mount(content)
  provider.onShow = () => { visible.value = true }
  provider.onHide = () => { visible.value = false }

  return {
    update(nextView: EditorView, prevState?: Parameters<SlashProvider['update']>[1]) {
      currentView.value = nextView
      provider.update(nextView, prevState)
    },
    destroy() {
      provider.destroy()
      app.unmount()
      content.remove()
    },
  }
}

export function configureSlashMenu(ctx: Ctx): void {
  ctx.set(slashMenuPlugin.key, {
    view: (view: EditorView) => createSlashMenuView(ctx, view),
  })
}
