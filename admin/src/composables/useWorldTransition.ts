import { gsap } from 'gsap'
import type { InjectionKey, Ref } from 'vue'

export interface WorldTransition {
  enterWorld(
    card: HTMLElement | null,
    siblings: HTMLElement[],
    navigate: () => Promise<void>,
  ): Promise<void>
  enterWorkspace(rail: HTMLElement | null, canvas: HTMLElement | null): Promise<void>
  swapCanvas(surface: HTMLElement | null, replace: () => Promise<void>): Promise<void>
  reset(): void
  dispose(): void
}

export const worldTransitionKey: InjectionKey<WorldTransition> = Symbol('world-transition')
export const writingRailKey: InjectionKey<Ref<HTMLElement | null>> = Symbol('writing-rail')

const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)'
const CLEARABLE_PROPERTIES = ['transform', 'opacity', 'visibility', 'will-change'] as const

export function useWorldTransition(root: Ref<HTMLElement | null>): WorldTransition {
  let context: ReturnType<typeof gsap.context> | null = null
  let matcher: ReturnType<typeof gsap.matchMedia> | null = null
  let activeTimeline: ReturnType<typeof gsap.timeline> | null = null
  let settleTimeline: (() => void) | null = null
  let prefersReducedMotion = false
  let transitionGeneration = 0
  let disposed = false
  const trackedElements = new Set<HTMLElement>()

  function ensureContext(): void {
    if (disposed || root.value === null || context !== null) return

    context = gsap.context(() => {}, root.value)
    matcher = gsap.matchMedia()
    matcher.add(
      { reduceMotion: REDUCED_MOTION_QUERY },
      (mediaContext) => {
        prefersReducedMotion = Boolean(mediaContext.conditions?.reduceMotion)
      },
      root.value,
    )
  }

  function runScoped(work: () => void): void {
    ensureContext()
    if (context === null) {
      work()
      return
    }

    context.add(work)
  }

  function track(targets: Array<HTMLElement | null>): void {
    for (const target of targets) {
      if (target !== null) trackedElements.add(target)
    }
  }

  function clearTrackedStyles(): void {
    for (const element of trackedElements) {
      for (const property of CLEARABLE_PROPERTIES) {
        element.style.removeProperty(property)
      }
    }
    trackedElements.clear()
  }

  function primeElement(
    element: HTMLElement | null,
    axis: 'x' | 'y',
    distance: number,
    hidden: boolean,
  ): void {
    if (element === null) return
    element.style.willChange = 'transform, opacity'
    element.style.opacity = hidden ? '0' : '1'
    element.style.visibility = hidden ? 'hidden' : 'visible'
    element.style.transform =
      axis === 'x' ? `translate3d(${distance}px, 0, 0)` : `translate3d(0, ${distance}px, 0)`
  }

  function killActiveTimeline(): void {
    if (activeTimeline === null) return
    activeTimeline.kill()
    activeTimeline = null
    settleTimeline?.()
    settleTimeline = null
    clearTrackedStyles()
  }

  function createTimeline(): ReturnType<typeof gsap.timeline> {
    killActiveTimeline()
    const timeline = gsap.timeline({ defaults: { overwrite: 'auto' } })
    activeTimeline = timeline
    return timeline
  }

  function nextGeneration(): number {
    transitionGeneration += 1
    return transitionGeneration
  }

  function isCurrent(generation: number): boolean {
    return !disposed && generation === transitionGeneration
  }

  function waitForTimeline(timeline: ReturnType<typeof gsap.timeline>): Promise<void> {
    return new Promise((resolve) => {
      let settled = false
      const settle = () => {
        if (settled) return
        settled = true
        if (settleTimeline === settle) settleTimeline = null
        if (activeTimeline === timeline) activeTimeline = null
        resolve()
      }

      settleTimeline = settle
      timeline.eventCallback('onComplete', settle)
      timeline.eventCallback('onInterrupt', settle)
    })
  }

  async function enterWorld(
    card: HTMLElement | null,
    siblings: HTMLElement[],
    navigate: () => Promise<void>,
  ): Promise<void> {
    if (disposed) return
    ensureContext()
    const generation = nextGeneration()
    const otherCards = siblings.filter((element) => element !== card)
    track([card, ...otherCards])

    if (card === null || prefersReducedMotion) {
      killActiveTimeline()
      await navigate()
      return
    }

    const timeline = createTimeline()
    runScoped(() => {
      timeline.addLabel('leave', 0)
      if (otherCards.length > 0) {
        timeline.to(
          otherCards,
          {
            autoAlpha: 0,
            y: 10,
            duration: 0.2,
            ease: 'power3.inOut',
            stagger: otherCards.length > 1 ? 0.04 : 0,
          },
          'leave',
        )
      }
      timeline.to(
        card,
        {
          autoAlpha: 1,
          scale: 1.018,
          duration: 0.24,
          ease: 'power3.inOut',
        },
        'leave',
      )
      timeline.addLabel('navigate')
      timeline.addLabel('enter')
    })

    await waitForTimeline(timeline)
    if (!isCurrent(generation) || root.value === null) return
    await navigate()
    clearTrackedStyles()
  }

  async function enterWorkspace(rail: HTMLElement | null, canvas: HTMLElement | null): Promise<void> {
    if (disposed) return
    ensureContext()
    const generation = nextGeneration()
    track([rail, canvas])

    if (prefersReducedMotion || canvas === null) {
      killActiveTimeline()
      clearTrackedStyles()
      return
    }

    primeElement(rail, 'x', -16, true)
    primeElement(canvas, 'y', 10, true)
    const timeline = createTimeline()

    runScoped(() => {
      timeline.addLabel('enter', 0)
      if (rail !== null) {
        timeline.to(
          rail,
          {
            autoAlpha: 1,
            x: 0,
            duration: 0.22,
            ease: 'power2.out',
          },
          'enter',
        )
      }
      timeline.to(
        canvas,
        {
          autoAlpha: 1,
          y: 0,
          duration: 0.24,
          ease: 'power2.out',
        },
        rail !== null ? 'enter+=0.08' : 'enter',
      )
    })

    await waitForTimeline(timeline)
    if (!isCurrent(generation)) return
    clearTrackedStyles()
  }

  async function swapCanvas(surface: HTMLElement | null, replace: () => Promise<void>): Promise<void> {
    if (disposed) return
    ensureContext()
    const generation = nextGeneration()
    track([surface])

    if (surface === null || prefersReducedMotion) {
      killActiveTimeline()
      await replace()
      clearTrackedStyles()
      return
    }

    surface.style.willChange = 'transform, opacity'
    const leaveTimeline = createTimeline()
    runScoped(() => {
      leaveTimeline.to(surface, {
        autoAlpha: 0,
        y: 8,
        duration: 0.15,
        ease: 'power2.out',
      })
    })

    await waitForTimeline(leaveTimeline)
    if (!isCurrent(generation)) return
    await replace()
    if (!isCurrent(generation)) return

    primeElement(surface, 'y', 8, true)
    const enterTimeline = createTimeline()
    runScoped(() => {
      enterTimeline.to(surface, {
        autoAlpha: 1,
        y: 0,
        duration: 0.24,
        ease: 'power2.out',
      })
    })

    await waitForTimeline(enterTimeline)
    if (!isCurrent(generation)) return
    clearTrackedStyles()
  }

  function dispose(): void {
    disposed = true
    transitionGeneration += 1
    killActiveTimeline()
    context?.revert()
    matcher?.revert()
    context = null
    matcher = null
  }

  function reset(): void {
    killActiveTimeline()
    clearTrackedStyles()
  }

  return {
    dispose,
    reset,
    enterWorld,
    enterWorkspace,
    swapCanvas,
  }
}
