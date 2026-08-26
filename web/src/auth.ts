import type { AuthLoginResponse, AuthSessionResponse } from './types'

export type AuthPhase = 'checking' | 'login' | 'enroll_totp' | 'verify_totp' | 'authenticated'

export interface AuthState {
  phase: AuthPhase
  authenticated: boolean
  username: string
  householdId: number | null
  csrfToken: string
  challenge: string
  totpSecret: string
  otpauthURI: string
  recoveryCodes: string[]
}

type AuthListener = (state: AuthState) => void

const listeners = new Set<AuthListener>()
let state: AuthState = emptyState('checking')

export function currentAuthState(): AuthState {
  return snapshot(state)
}

export function subscribeAuthState(listener: AuthListener): () => void {
  listeners.add(listener)
  listener(currentAuthState())
  return () => listeners.delete(listener)
}

export function getCSRFToken(): string {
  return state.authenticated ? state.csrfToken : ''
}

export function clearAuthState(): void {
  replaceState(emptyState('login'))
}

export function clearRecoveryCodes(): void {
  if (!state.recoveryCodes.length) return
  replaceState({ ...state, recoveryCodes: [] })
}

export async function bootstrapSession(): Promise<AuthState> {
  const session = await authRequestJSON<AuthSessionResponse>('/api/v1/auth/session')
  if (!session.authenticated) {
    clearAuthState()
    return currentAuthState()
  }
  replaceState(authenticatedState(session, []))
  return currentAuthState()
}

export async function beginLogin(username: string, password: string): Promise<AuthState> {
  const response = await authRequestJSON<AuthLoginResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
  replaceState({
    ...emptyState(response.step),
    challenge: response.challenge,
    totpSecret: response.totp_secret ?? '',
    otpauthURI: response.otpauth_uri ?? '',
  })
  return currentAuthState()
}

export async function confirmTOTP(code: string): Promise<AuthState> {
  if (state.phase !== 'enroll_totp' || !state.challenge) throw new Error('invalid_auth_state')
  const challenge = state.challenge
  const issue = await authRequestJSON<AuthSessionResponse>('/api/v1/auth/totp/enroll/confirm', {
    method: 'POST',
    body: JSON.stringify({ challenge, code }),
  })
  return finishSession(issue)
}

export async function verifySecondFactor(code: string, recovery = false): Promise<AuthState> {
  if (state.phase !== 'verify_totp' || !state.challenge) throw new Error('invalid_auth_state')
  const challenge = state.challenge
  const issue = await authRequestJSON<AuthSessionResponse>('/api/v1/auth/verify', {
    method: 'POST',
    body: JSON.stringify({ challenge, code, recovery }),
  })
  return finishSession(issue)
}

export async function logout(): Promise<void> {
  const headers = new Headers()
  if (state.csrfToken) headers.set('X-CSRF-Token', state.csrfToken)
  const response = await fetch('/api/v1/auth/logout', {
    method: 'POST',
    headers,
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (response.status === 401) {
    clearAuthState()
    return
  }
  if (!response.ok) throw new Error(await stableErrorCode(response))
  clearAuthState()
}

async function finishSession(issue: AuthSessionResponse): Promise<AuthState> {
  if (!issue.authenticated || !issue.csrf_token) throw new Error('unauthenticated')
  const recoveryCodes = issue.recovery_codes ?? []
  const session = await authRequestJSON<AuthSessionResponse>('/api/v1/auth/session')
  if (!session.authenticated) {
    clearAuthState()
    throw new Error('unauthenticated')
  }
  replaceState(authenticatedState(session, recoveryCodes))
  return currentAuthState()
}

function authenticatedState(session: AuthSessionResponse, recoveryCodes: string[]): AuthState {
  if (!session.csrf_token || !Number.isSafeInteger(session.household_id) || (session.household_id ?? 0) <= 0) {
    throw new Error('invalid_session')
  }
  return {
    phase: 'authenticated',
    authenticated: true,
    username: session.username ?? '',
    householdId: session.household_id ?? null,
    csrfToken: session.csrf_token,
    challenge: '',
    totpSecret: '',
    otpauthURI: '',
    recoveryCodes: [...recoveryCodes],
  }
}

async function authRequestJSON<T>(url: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  headers.set('Accept', 'application/json')
  const response = await fetch(url, {
    ...init,
    headers,
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (!response.ok) throw new Error(await stableErrorCode(response))
  return (await response.json()) as T
}

async function stableErrorCode(response: Response): Promise<string> {
  try {
    const envelope = (await response.json()) as { error?: { code?: string } }
    if (envelope.error?.code) return envelope.error.code
  } catch {
    // Use the stable fallback below.
  }
  return 'request_failed'
}

function replaceState(next: AuthState): void {
  state = next
  for (const listener of listeners) listener(currentAuthState())
}

function emptyState(phase: AuthPhase): AuthState {
  return {
    phase,
    authenticated: false,
    username: '',
    householdId: null,
    csrfToken: '',
    challenge: '',
    totpSecret: '',
    otpauthURI: '',
    recoveryCodes: [],
  }
}

function snapshot(value: AuthState): AuthState {
  return { ...value, recoveryCodes: [...value.recoveryCodes] }
}
