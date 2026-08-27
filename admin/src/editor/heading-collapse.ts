const HEADING_SELECTOR = 'h1, h2, h3, h4, h5, h6'

function levelOf(heading: HTMLHeadingElement): number {
  return Number(heading.tagName.slice(1))
}

export function toggleHeadingSection(heading: HTMLHeadingElement): void {
  const collapsed = heading.dataset.headingCollapsed === 'true'
  const level = levelOf(heading)
  let sibling = heading.nextElementSibling

  heading.dataset.headingCollapsed = collapsed ? 'false' : 'true'
  if (collapsed) delete heading.dataset.headingCollapsed
  heading.setAttribute('aria-expanded', collapsed ? 'true' : 'false')

  while (sibling !== null) {
    const next = sibling.nextElementSibling
    const childHeading = sibling.matches(HEADING_SELECTOR) ? (sibling as HTMLHeadingElement) : null
    if (childHeading !== null && levelOf(childHeading) <= level) break
    sibling.toggleAttribute('hidden', !collapsed)
    sibling = next
  }
}

export function syncHeadingSections(root: HTMLElement): void {
  const headings = root.querySelectorAll<HTMLHeadingElement>(HEADING_SELECTOR)
  for (const heading of headings) {
    heading.tabIndex = 0
    heading.setAttribute('role', 'button')
    heading.setAttribute('aria-expanded', heading.dataset.headingCollapsed === 'true' ? 'false' : 'true')
  }
}
