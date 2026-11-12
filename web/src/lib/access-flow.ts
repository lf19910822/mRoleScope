export const accessVerifyPath = '/api/v1/access/verify'
export const accessCheckPath = '/api/v1/access/check'

export type AccessVerificationResult = 'verified' | 'invalid' | 'rate_limited' | 'failed'

export async function verifyInvite(
  inviteCode: string,
  fetcher: typeof fetch = fetch,
): Promise<AccessVerificationResult> {
  try {
    const response = await fetcher(accessVerifyPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ invite_code: inviteCode }),
    })
    if (response.status === 204) return 'verified'
    if (response.status === 401) return 'invalid'
    if (response.status === 429) return 'rate_limited'
    return 'failed'
  } catch {
    return 'failed'
  }
}

export async function hasAccess(fetcher: typeof fetch = fetch): Promise<boolean> {
  try {
    const response = await fetcher(accessCheckPath, { headers: { Accept: 'application/json' } })
    return response.status === 204
  } catch {
    return false
  }
}
