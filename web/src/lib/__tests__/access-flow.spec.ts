import { describe, expect, it, vi } from 'vitest'

import { accessCheckPath, accessVerifyPath, hasAccess, verifyInvite } from '@/lib/access-flow'

describe('access flow', () => {
  it('uses anonymous same-origin routes and reports successful verification', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(null, { status: 204 }))

    await expect(verifyInvite('known-invite', fetcher)).resolves.toBe('verified')
    expect(fetcher).toHaveBeenCalledWith(accessVerifyPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ invite_code: 'known-invite' }),
    })
  })

  it('keeps failure messages generic while distinguishing rate limiting', async () => {
    await expect(
      verifyInvite(
        'wrong',
        vi.fn<typeof fetch>().mockResolvedValue(new Response(null, { status: 401 })),
      ),
    ).resolves.toBe('invalid')
    await expect(
      verifyInvite(
        'wrong',
        vi.fn<typeof fetch>().mockResolvedValue(new Response(null, { status: 429 })),
      ),
    ).resolves.toBe('rate_limited')
  })

  it('treats a valid three-day browser session as a successful revisit', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(null, { status: 204 }))

    await expect(hasAccess(fetcher)).resolves.toBe(true)
    expect(fetcher).toHaveBeenCalledWith(accessCheckPath, {
      headers: { Accept: 'application/json' },
    })
  })

  it('does not treat API or network failures as an authorized session', async () => {
    await expect(
      hasAccess(vi.fn<typeof fetch>().mockResolvedValue(new Response(null, { status: 401 }))),
    ).resolves.toBe(false)
    await expect(
      hasAccess(vi.fn<typeof fetch>().mockRejectedValue(new Error('offline'))),
    ).resolves.toBe(false)
  })
})
