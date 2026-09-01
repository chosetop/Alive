import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import JournalCanvas from './JournalCanvas.vue'

vi.mock('../MarkdownEditor.vue', async () => {
  const { defineComponent } = await import('vue')
  return {
    default: defineComponent({
      name: 'MarkdownEditor',
      props: {
        initialValue: { type: String, required: true },
        disabled: { type: Boolean, default: false },
      },
      emits: ['update'],
      template:
        '<textarea aria-label="正文编辑器" :value="initialValue" :disabled="disabled" @input="$emit(\'update\', $event.target.value)" />',
    }),
  }
})

describe('JournalCanvas', () => {
  it('renders a manuscript title field and forwards title edits', async () => {
    const wrapper = mount(JournalCanvas, {
      props: {
        title: '原始标题',
        content: '# 原始正文',
      },
    })

    expect(wrapper.get('[data-journal-canvas]').element).toBeTruthy()
    expect(wrapper.get('[data-journal-title]').attributes('aria-label')).toBe('日志标题')

    await wrapper.get('[data-journal-title]').setValue('一场缓慢的夏雨')

    expect(wrapper.emitted('update:title')).toEqual([['一场缓慢的夏雨']])
  })

  it('keeps markdown editing inside the canvas and emits content updates', async () => {
    const wrapper = mount(JournalCanvas, {
      props: {
        title: '原始标题',
        content: '# 原始正文',
      },
    })

    const editor = wrapper.get('textarea[aria-label="正文编辑器"]')
    await editor.setValue('新的正文')

    expect(wrapper.emitted('update:content')).toEqual([['新的正文']])
  })

  it('passes the disabled state through to the editor controls', () => {
    const wrapper = mount(JournalCanvas, {
      props: {
        title: '原始标题',
        content: '# 原始正文',
        disabled: true,
      },
    })

    expect(wrapper.get('[data-journal-title]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('textarea[aria-label="正文编辑器"]').attributes('disabled')).toBeDefined()
  })
})
