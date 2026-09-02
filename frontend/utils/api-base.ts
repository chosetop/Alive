export function resolveApiBase(
  configuredOrigin: string,
  isServer: boolean,
  browserOrigin = '',
  serverOrigin = '',
): string {
  if (isServer && serverOrigin) return serverOrigin
  if (!isServer && configuredOrigin) {
    try {
      const configured = new URL(configuredOrigin)
      const browser = new URL(browserOrigin)
      const isLocalApi = configured.hostname === 'localhost' || configured.hostname === '127.0.0.1'

      if (isLocalApi && browser.hostname !== 'localhost' && browser.hostname !== '127.0.0.1') {
        return `${browser.protocol}//${browser.hostname}:${configured.port || '80'}`
      }
    } catch {
      // Fall through to the configured origin when either value is not a URL.
    }
  }
  if (configuredOrigin) return configuredOrigin
  if (isServer) return 'http://127.0.0.1:8081'
  try {
    const url = new URL(browserOrigin)
    if (url.hostname === 'localhost' && url.port === '3000') return 'http://127.0.0.1:8080'
  } catch {
    // An empty origin is only possible outside a browser; leave it unchanged.
  }
  return browserOrigin
}
