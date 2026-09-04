import { onBeforeUnmount, onMounted, type Ref } from 'vue'
import { gsap } from 'gsap'

export function useSayingMotion(root: Ref<HTMLElement | null>): void {
  let observer: IntersectionObserver | null = null
  let context: ReturnType<typeof gsap.context> | null = null

  onMounted(() => {
    if (!root.value || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    context = gsap.context(() => {
      const items = root.value?.querySelectorAll<HTMLElement>('[data-saying-id]') ?? []
      observer = new IntersectionObserver((entries) => {
        for (const entry of entries) {
          const target = entry.target as HTMLElement
          if (entry.isIntersecting) gsap.to(target, { autoAlpha: 1, y: 0, duration: 0.45, ease: 'power2.out', overwrite: true })
          else gsap.to(target, { autoAlpha: 0.82, y: 18, duration: 0.3, ease: 'power2.out', overwrite: true })
        }
      }, { threshold: 0.05 })
      items.forEach((item) => {
        gsap.set(item, { autoAlpha: 0, y: 18 })
        observer?.observe(item)
      })
    }, root)
  })

  onBeforeUnmount(() => {
    observer?.disconnect()
    observer = null
    context?.revert()
    context = null
  })
}
