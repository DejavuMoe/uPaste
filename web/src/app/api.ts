import type {
  CreateEncryptedTextRequest,
  CreateFileMetadata,
  CreateShareResponse,
  CreateStandardTextRequest,
  GetShareResponse,
  UploadProgress,
  SharePatch,
  UpdateShareResponse,
} from './types'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly retryAfterSeconds?: number

  constructor(status: number, code: string, message: string, retryAfterSeconds?: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.retryAfterSeconds = retryAfterSeconds
  }
}

export function parseRetryAfter(headerValue: string | null): number | undefined {
  if (!headerValue) return undefined
  const parsedInt = parseInt(headerValue, 10)
  if (!isNaN(parsedInt) && parsedInt >= 0) {
    return parsedInt
  }
  // Try parsing as HTTP-date
  const parsedDate = Date.parse(headerValue)
  if (!isNaN(parsedDate)) {
    const diffSeconds = Math.ceil((parsedDate - Date.now()) / 1000)
    return Math.max(0, diffSeconds)
  }
  return undefined
}

async function handleResponse<T>(res: Response): Promise<T> {
  const rawText = await res.text()

  if (res.ok) {
    return JSON.parse(rawText) as T
  }

  const retryAfterSeconds = parseRetryAfter(res.headers.get('Retry-After'))
  let code = 'unknown_error'
  let message = `Request failed with status ${res.status}`

  try {
    const body = JSON.parse(rawText)
    if (body && typeof body === 'object' && body.error) {
      code = body.error.code || code
      message = body.error.message || message
    }
  } catch {
    if (rawText.trim()) {
      message = rawText.trim()
    }
  }

  throw new ApiError(res.status, code, message, retryAfterSeconds)
}

export async function createStandardText(
  req: CreateStandardTextRequest,
  signal?: AbortSignal,
): Promise<CreateShareResponse> {
  const res = await fetch('/api/v1/shares', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(req),
    signal,
  })
  return handleResponse<CreateShareResponse>(res)
}

export async function createEncryptedText(
  req: CreateEncryptedTextRequest,
  signal?: AbortSignal,
): Promise<CreateShareResponse> {
  const res = await fetch('/api/v1/shares', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(req),
    signal,
  })
  return handleResponse<CreateShareResponse>(res)
}

export async function getShare(
  id: string,
  signal?: AbortSignal,
): Promise<GetShareResponse> {
  const res = await fetch(`/api/v1/shares/${encodeURIComponent(id)}`, {
    method: 'GET',
    signal,
  })
  return handleResponse<GetShareResponse>(res)
}

export async function updateShare(
  id: string,
  ownerToken: string,
  patch: SharePatch,
  signal?: AbortSignal,
): Promise<UpdateShareResponse> {
  const res = await fetch(`/api/v1/shares/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${ownerToken}` },
    body: JSON.stringify(patch),
    signal,
  })
  return handleResponse<UpdateShareResponse>(res)
}

export async function deleteShare(id: string, ownerToken: string, signal?: AbortSignal): Promise<void> {
  const res = await fetch(`/api/v1/shares/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${ownerToken}` },
    signal,
  })
  if (res.status === 204) return
  await handleResponse<void>(res)
}

export function createFile(
  file: File,
  expiresAt: string | null,
  onProgress?: (progress: UploadProgress) => void,
  signal?: AbortSignal,
): Promise<CreateShareResponse> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()

    if (signal) {
      if (signal.aborted) {
        reject(new DOMException('Upload aborted', 'AbortError'))
        return
      }
      signal.addEventListener(
        'abort',
        () => {
          xhr.abort()
          reject(new DOMException('Upload aborted', 'AbortError'))
        },
        { once: true },
      )
    }

    xhr.open('POST', '/api/v1/shares')

    if (xhr.upload && onProgress) {
      xhr.upload.onprogress = (e: ProgressEvent) => {
        const lengthComputable = e.lengthComputable
        const percent = lengthComputable && e.total > 0 ? Math.round((e.loaded / e.total) * 100) : null
        onProgress({
          loaded: e.loaded,
          total: e.total,
          lengthComputable,
          percent,
        })
      }
    }

    xhr.onload = () => {
      const retryAfter = parseRetryAfter(xhr.getResponseHeader('Retry-After'))
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          const data = JSON.parse(xhr.responseText) as CreateShareResponse
          resolve(data)
        } catch (err) {
          reject(new ApiError(xhr.status, 'invalid_response', 'Failed to parse server response'))
        }
      } else {
        let code = 'unknown_error'
        let message = `Upload failed with status ${xhr.status}`
        try {
          const body = JSON.parse(xhr.responseText)
          if (body && body.error) {
            code = body.error.code || code
            message = body.error.message || message
          }
        } catch {
          if (xhr.responseText.trim()) {
            message = xhr.responseText.trim()
          }
        }
        reject(new ApiError(xhr.status, code, message, retryAfter))
      }
    }

    xhr.onerror = () => {
      reject(new ApiError(0, 'network_error', 'Could not reach the server'))
    }

    const formData = new FormData()
    const metadata: CreateFileMetadata = {
      payload_kind: 'FILE',
      privacy_mode: 'STANDARD',
      expires_at: expiresAt,
    }

    formData.append(
      'metadata',
      new Blob([JSON.stringify(metadata)], { type: 'application/json' }),
    )
    formData.append('file', file)

    xhr.send(formData)
  })
}
