// @ts-check
import prettier from 'eslint-config-prettier'
import withNuxt from './.nuxt/eslint.config.mjs'

/**
 * Nuxt generates the base config from the project's own structure, so it stays
 * in step with the auto-imports and the module set.
 *
 * `eslint-config-prettier` goes last: it switches off the stylistic rules that
 * would otherwise disagree with Prettier's output.
 */
export default withNuxt(prettier)
