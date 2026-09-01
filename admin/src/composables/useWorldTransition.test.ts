import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

import { useWorldTransition } from './useWorldTransition'

type MockTimeline = {
  addLabel: ReturnType<typeof vi.fn>
  eventCallback: ReturnType<typeof vi.fn>
  kill: ReturnType<typeof vi.fn>
  to: ReturnType<typeof vi.fn>
  callbacks: Record<string, (() => void) | undefined>
  complete: () => void
}

const gsapHarness = vi.hoisted(() => {
  const contexts: Array<{ add: ReturnType<typeof vi.fn>; revert: ReturnType<typeof vi.fn> }> = []
  const matchers: Array<{ add: ReturnType<typeof vi.fn>; revert: ReturnType<typeof vi.fn> }> = []
  const timelines: MockTimeline[] = []
  const state = {
    reduceMotion: false,
    reset() {
      state.reduceMotion = false
      contexts.length = 0
      matchers.length = 0
      timelines.length = 0
      gsapMock.context.mockClear()
      gsapMock.matchMedia.mockClear()
      gsapMock.timeline.mockClear()
    },
  }

  const createTimeline = (): MockTimeline => {
    const callbacks: Record<string, (() => void) | undefined> = {}
    const timeline: MockTimeline = {
      addLabel: vi.fn(),
      eventCallback: vi.fn((name: string, callback: () => void) => {
        callbacks[name] = callback
        return timeline
      }),
      kill: vi.fn(() => {
        callbacks.onInterrupt?.()
      }),
      to: vi.fn(),
      callbacks,
      complete: () => {
        callbacks.onComplete?.()
      },
    }
    timeline.addLabel.mockReturnValue(timeline)
    timeline.to.mockReturnValue(timeline)
    timelines.push(timeline)
    return timeline
  }

  const gsapMock = {
    context: vi.fn((callback: () => void, _scope?: Element | null) => {
      callback()
      const context = {
        add: vi.fn((fn: () => void) => {
          fn()
          return fn
        }),
        revert: vi.fn(),
      }
      contexts.push(context)
      return context
    }),
    matchMedia: vi.fn(() => {
      const matcher = {
        add: vi.fn((_query: unknown, callback: (context: { conditions: { reduceMotion: boolean } }) => void) => {
          callback({ conditions: { reduceMotion: state.reduceMotion } })
        }),
        revert: vi.fn(),
      }
      matchers.push(matcher)
      return matcher
    }),
    timeline: vi.fn(() => createTimeline()),
  }

  return {
    contexts,
    gsapMock,
    matchers,
    state,
    timelines,
  }
})

vi.mock('gsap', () => ({ gsap: gsapHarness.gsapMock }))

function mountScope() {
  const root = document.createElement('section')
  const card = document.createElement('article')
  const sibling = document.createElement('article')
  const secondSibling = document.createElement('article')
  root.append(card, sibling, secondSibling)
  document.body.append(root)
  return {
    card,
    root,
    secondSibling,
    sibling,
    transition: useWorldTransition(ref(root)),
  }
}

describe('useWorldTransition', () => {
  beforeEach(() => {
    gsapHarness.state.reset()
    document.body.innerHTML = ''
  })

  it('builds the world-entry leave timeline with labels and approved motion properties', async () => {
    const { card, sibling, secondSibling, transition } = mountScope()
    const navigate = vi.fn().mockResolvedValue(undefined)

    const pending = transition.enterWorld(card, [sibling, secondSibling], navigate)

    expect(gsapHarness.gsapMock.timeline).toHaveBeenCalledOnce()
    const timeline = gsapHarness.timelines[0]
    expect(timeline.addLabel).toHaveBeenCalledWith('leave', 0)
    expect(timeline.addLabel).toHaveBeenCalledWith('navigate')
    expect(timeline.addLabel).toHaveBeenCalledWith('enter')
    expect(timeline.to).toHaveBeenCalledWith(
      [sibling, secondSibling],
      expect.objectContaining({ autoAlpha: 0, y: 10, stagger: 0.04 }),
      'leave',
    )
    expect(timeline.to).toHaveBeenCalledWith(
      card,
      expect.objectContaining({ autoAlpha: 1, scale: 1.018 }),
      'leave',
    )

    for (const [, vars] of timeline.to.mock.calls) {
      const animatedKeys = Object.keys(vars as Record<string, unknown>).filter(
        (key) => !['duration', 'ease', 'overwrite', 'stagger'].includes(key),
      )
      expect(animatedKeys.every((key) => ['autoAlpha', 'scale', 'x', 'y'].includes(key))).toBe(true)
    }

    expect(navigate).not.toHaveBeenCalled()
    timeline.complete()
    await pending
    expect(navigate).toHaveBeenCalledOnce()
  })

  it('skips positional motion and stagger when reduced motion is enabled', async () => {
    gsapHarness.state.reduceMotion = true
    const { card, sibling, transition } = mountScope()
    const navigate = vi.fn().mockResolvedValue(undefined)

    await transition.enterWorld(card, [sibling], navigate)

    expect(navigate).toHaveBeenCalledOnce()
    expect(gsapHarness.gsapMock.timeline).not.toHaveBeenCalled()
  })

  it('kills the active timeline, reverts the scoped context, and clears inline styles on dispose', async () => {
    const { root, transition } = mountScope()
    const surface = document.createElement('div')
    root.append(surface)
    const replace = vi.fn().mockResolvedValue(undefined)

    const pending = transition.swapCanvas(surface, replace)
    const timeline = gsapHarness.timelines[0]

    surface.style.transform = 'translateY(8px)'
    surface.style.opacity = '0'
    surface.style.visibility = 'hidden'

    transition.dispose()
    await pending

    expect(timeline.kill).toHaveBeenCalledOnce()
    expect(gsapHarness.contexts.at(-1)?.revert).toHaveBeenCalledOnce()
    expect(gsapHarness.matchers.at(-1)?.revert).toHaveBeenCalledOnce()
    expect(surface.style.transform).toBe('')
    expect(surface.style.opacity).toBe('')
    expect(surface.style.visibility).toBe('')
  })
})
