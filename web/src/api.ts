import { clearAuthState, getCSRFToken } from './auth.ts'
import type {
  AdvisorResponse,
  CreateHouseholdMemberRequest,
  DashboardData,
  HouseholdMemberResponse,
  HouseholdMembersResponse,
  HouseholdRole,
  PortfolioAssetResponse,
  PortfolioAssetsResponse,
  PortfolioAssetUpsertRequest,
} from './types'

interface APIErrorEnvelope {
  error?: {
    code?: string
    message?: string
  }
}

export async function loadDashboard(period: string): Promise<DashboardData> {
  const month = encodeURIComponent(period)
  return requestJSON<DashboardData>(`/api/v1/dashboard?period=${month}`)
}

export async function askAdvisor(question: string, requireReview: boolean): Promise<AdvisorResponse> {
  return requestJSON<AdvisorResponse>('/api/v1/advisor', {
    method: 'POST',
    body: JSON.stringify({
      question,
      require_tool: true,
      require_review: requireReview,
    }),
  })
}

export async function listPortfolioAssets(): Promise<PortfolioAssetsResponse> {
  return requestJSON<PortfolioAssetsResponse>('/api/v1/portfolio/assets')
}

export async function upsertPortfolioAsset(
  assetRef: string,
  request: PortfolioAssetUpsertRequest,
): Promise<PortfolioAssetResponse> {
  const encodedAssetRef = encodeURIComponent(assetRef)
  return requestJSON<PortfolioAssetResponse>(`/api/v1/portfolio/assets/${encodedAssetRef}`, {
    method: 'PUT',
    body: JSON.stringify(request),
  })
}

export async function deletePortfolioAsset(assetRef: string): Promise<void> {
  const encodedAssetRef = encodeURIComponent(assetRef)
  await requestResponse(`/api/v1/portfolio/assets/${encodedAssetRef}`, {
    method: 'DELETE',
  })
}

export async function listHouseholdMembers(): Promise<HouseholdMembersResponse> {
  return requestJSON<HouseholdMembersResponse>('/api/v1/household/members')
}

export async function createHouseholdMember(request: CreateHouseholdMemberRequest): Promise<HouseholdMemberResponse> {
  return requestJSON<HouseholdMemberResponse>('/api/v1/household/members', {
    method: 'POST',
    body: JSON.stringify(request),
  })
}

export async function updateHouseholdMemberRole(userId: number, role: HouseholdRole): Promise<HouseholdMemberResponse> {
  return requestJSON<HouseholdMemberResponse>(`/api/v1/household/members/${encodeURIComponent(String(userId))}`, {
    method: 'PATCH',
    body: JSON.stringify({ role }),
  })
}

export async function disableHouseholdMember(userId: number): Promise<void> {
  await requestResponse(`/api/v1/household/members/${encodeURIComponent(String(userId))}`, {
    method: 'DELETE',
  })
}

async function requestJSON<T>(url: string, init: RequestInit = {}): Promise<T> {
  const response = await requestResponse(url, init)
  return (await response.json()) as T
}

async function requestResponse(url: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  headers.set('Accept', 'application/json')

  const method = (init.method ?? 'GET').toUpperCase()
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const csrf = getCSRFToken()
    if (csrf) headers.set('X-CSRF-Token', csrf)
  }

  const response = await fetch(url, {
    ...init,
    headers,
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === httpUnauthorized) {
    clearAuthState()
    throw new Error('unauthenticated')
  }
  if (!response.ok) {
    let code = 'request_failed'
    try {
      const envelope = (await response.json()) as APIErrorEnvelope
      if (envelope.error?.code) code = envelope.error.code
    } catch {
      // Keep the stable generic code; never surface an untrusted response body.
    }
    throw new Error(code)
  }
  return response
}

const httpUnauthorized = 401
