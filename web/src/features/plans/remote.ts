import type { FlowCandidate } from './types.ts'

export const REMOTE_SEARCH_DEBOUNCE_MS = 250

export interface RemoteRequestIdentity {
  version: number
  account: string
  source: string
  query: string
}

export function candidateIdentity(candidate: FlowCandidate): string {
  if (candidate.kind === 'template') return `template:${candidate.templateId}`
  if (candidate.kind === 'submitted') return `submitted:${candidate.id}`
  return `due:${candidate.flowInstanceId}`
}

export function mergeCandidatePages(existing: readonly FlowCandidate[], incoming: readonly FlowCandidate[]): FlowCandidate[] {
  const merged = new Map<string, FlowCandidate>()
  for (const candidate of existing) merged.set(candidateIdentity(candidate), candidate)
  for (const candidate of incoming) merged.set(candidateIdentity(candidate), candidate)
  return [...merged.values()]
}

export function isCurrentRemoteRequest(current: RemoteRequestIdentity, response: RemoteRequestIdentity): boolean {
  return current.version === response.version
    && current.account === response.account
    && current.source === response.source
    && current.query === response.query
}

export function retryPageFor(items: readonly FlowCandidate[], currentPage: number, failedPage: number | null): number {
  if (failedPage !== null) return failedPage
  return items.length === 0 ? 1 : currentPage + 1
}

export function invalidatesVerification(code: string | undefined): boolean {
  return code === 'TARGET_SESSION_EXPIRED' || code === 'TARGET_LOGIN_REJECTED'
}

export function createDebouncedRunner<T>(callback: (value: T) => void, delay = REMOTE_SEARCH_DEBOUNCE_MS) {
  let timer: ReturnType<typeof setTimeout> | null = null
  return {
    schedule(value: T) {
      if (timer !== null) clearTimeout(timer)
      timer = setTimeout(() => {
        timer = null
        callback(value)
      }, delay)
    },
    cancel() {
      if (timer !== null) clearTimeout(timer)
      timer = null
    },
  }
}
