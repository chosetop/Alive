export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig(event)
  const base = String(config.public.apiBase || 'http://localhost:8080/api/v1').replace(/\/$/, '')
  const origin = getRequestURL(event).origin
  const urls = [`${origin}/`, `${origin}/journal`, `${origin}/sayings`]
  try {
    const journals = await $fetch<{ data?: Array<{ slug: string }> }>(`${base}/journals`)
    for (const item of journals.data ?? []) urls.push(`${origin}/journal/${encodeURIComponent(item.slug)}`)
  } catch { /* sitemap remains useful with stable public roots when API is unavailable */ }
  const body = urls.map((url) => `<url><loc>${escapeXml(url)}</loc></url>`).join('')
  setHeader(event, 'content-type', 'application/xml; charset=utf-8')
  return `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${body}</urlset>`
})

function escapeXml(value: string): string {
  return value.replace(/[<>&'\"]/g, (char) => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;', "'": '&apos;', '"': '&quot;' })[char] ?? char)
}
