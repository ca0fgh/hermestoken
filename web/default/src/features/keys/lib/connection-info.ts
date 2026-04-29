type ApiInfoLike = {
  url?: unknown
}

type StatusLike = {
  api_info?: unknown
  server_address?: unknown
  data?: unknown
}

function trimTrailingSlashes(value: string): string {
  return value.trim().replace(/\/+$/, '')
}

function readStatusFromLocalStorage(): StatusLike | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.localStorage.getItem('status')
    if (!raw) return null
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? (parsed as StatusLike) : null
  } catch {
    return null
  }
}

function normalizeStatusPayload(status: StatusLike | null): StatusLike | null {
  if (!status) return null
  const data = status.data
  if (data && typeof data === 'object' && !Array.isArray(data)) {
    return data as StatusLike
  }
  return status
}

function firstApiInfoURL(status: StatusLike | null): string {
  const payload = normalizeStatusPayload(status)
  const apiInfo = Array.isArray(payload?.api_info) ? payload.api_info : []
  for (const item of apiInfo as ApiInfoLike[]) {
    if (typeof item?.url !== 'string') continue
    const url = trimTrailingSlashes(item.url)
    if (url) return url
  }
  return ''
}

export function getConfiguredConnectionURL(status?: StatusLike | null): string {
  const payload = normalizeStatusPayload(status ?? readStatusFromLocalStorage())
  const apiInfoURL = firstApiInfoURL(payload)
  if (apiInfoURL) return apiInfoURL
  if (typeof payload?.server_address === 'string') {
    return trimTrailingSlashes(payload.server_address)
  }
  if (typeof window === 'undefined') return ''
  return trimTrailingSlashes(window.location.origin)
}

export function encodeConnectionString(key: string, url: string): string {
  return JSON.stringify({
    _type: 'hermestoken_channel_conn',
    key,
    url,
  })
}
