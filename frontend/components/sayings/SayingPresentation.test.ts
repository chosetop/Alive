import { describe, expect, it } from 'vitest'
import streamSource from './SayingStream.vue?raw'
import wallSource from './SayingWall.vue?raw'
import focusSource from './SayingFocus.vue?raw'
import actionsSource from './SayingActions.vue?raw'
import indexSource from '../../pages/sayings/index.vue?raw'
import categorySource from '../../pages/sayings/categories/[slug].vue?raw'

describe('saying presentation', () => {
  it('removes immersive mode and all click-to-open preview wiring', () => {
    expect(indexSource).not.toContain("{ view: 'focus'")
    expect(categorySource).not.toContain("{ view: 'focus'")
    expect(indexSource).not.toContain('SayingPreview')
    expect(categorySource).not.toContain('SayingPreview')
    expect(streamSource).not.toContain('defineEmits')
    expect(wallSource).not.toContain('defineEmits')
    expect(focusSource).not.toContain('defineEmits')
  })

  it('keeps copy as a hover-revealed icon button', () => {
    expect(actionsSource).toContain('复制正文')
    expect(actionsSource).toContain('opacity: 0')
    expect(actionsSource).toContain('opacity: 1')
    expect(actionsSource).not.toContain('导出图片')
  })
})
