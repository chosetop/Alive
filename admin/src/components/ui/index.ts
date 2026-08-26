/**
 * The Alive UI primitive layer.
 *
 * Business components import from here and never from `reka-ui` directly. That is
 * the boundary the plan asks for, and it earns its keep in two ways: the ARIA and
 * focus decisions live in one place rather than being re-derived per call site,
 * and a Reka major version can be absorbed by editing six files instead of every
 * view.
 *
 * These are thin on purpose. This is not a UI kit — Reka supplies behaviour, Alive
 * supplies the visual rules in `ui.css`, and anything with product opinions
 * belongs in `components/writing/` instead.
 */
export { default as UiButton } from './UiButton.vue'
export { default as UiDialog } from './UiDialog.vue'
export { default as UiIconButton } from './UiIconButton.vue'
export { default as UiMenu } from './UiMenu.vue'
export { default as UiPopover } from './UiPopover.vue'
export { default as UiToastRegion } from './UiToastRegion.vue'

export type { UiButtonProps } from './UiButton.vue'
export type { UiDialogProps } from './UiDialog.vue'
export type { UiIconButtonProps } from './UiIconButton.vue'
export type { UiMenuItem, UiMenuProps } from './UiMenu.vue'
export type { UiPopoverProps } from './UiPopover.vue'
export type { UiToast } from './UiToastRegion.vue'
