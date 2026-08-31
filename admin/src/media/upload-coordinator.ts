import { presignMedia, registerMedia, type MediaRecord } from '../api/media'

export interface UploadOptions { entryId: number; onProgress?: (percent: number) => void; maxAttempts?: number }

function putObject(url: string, method: string, headers: Record<string, string>, file: Blob, onProgress?: (percent: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open(method || 'PUT', url)
    for (const [key, value] of Object.entries(headers)) xhr.setRequestHeader(key, value)
    xhr.upload.onprogress = (event) => { if (event.lengthComputable) onProgress?.(Math.round((event.loaded / event.total) * 100)) }
    xhr.onerror = () => reject(new Error('media upload failed'))
    xhr.onabort = () => reject(new Error('media upload aborted'))
    xhr.onload = () => xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`media upload returned ${xhr.status}`))
    xhr.send(file)
  })
}

export async function uploadMedia(file: File, options: UploadOptions): Promise<MediaRecord> {
  const attempts = Math.max(1, options.maxAttempts ?? 3)
  let lastError: unknown
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      const signed = await presignMedia({ entry_id: options.entryId, filename: file.name, mime_type: file.type, size_bytes: file.size })
      await putObject(signed.upload.url, signed.upload.method, signed.upload.headers, file, options.onProgress)
      return await registerMedia({ entry_id: options.entryId, object_key: signed.object_key, mime_type: file.type, size_bytes: file.size })
    } catch (error) {
      lastError = error
      if (attempt + 1 < attempts) continue
    }
  }
  throw lastError instanceof Error ? lastError : new Error('media upload failed')
}
