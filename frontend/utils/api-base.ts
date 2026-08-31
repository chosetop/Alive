export function resolveApiBase(
  configuredOrigin: string,
  isServer: boolean,
  browserOrigin = '',
): string {
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
