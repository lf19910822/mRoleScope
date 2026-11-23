import { afterEach, describe, expect, it, vi } from 'vitest'

import { createAITrendDraftState } from '../ai-trend-draft-state'

describe('AI trend draft state', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('consumes the backend NDJSON stream and stores its draft', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        new Response(
          [
            JSON.stringify({
              type: 'operation',
              status: 'running',
              tool: 'analyze_intent',
            }),
            JSON.stringify({
              type: 'operation',
              status: 'completed',
              tool: 'analyze_intent',
            }),
            JSON.stringify({
              type: 'operation',
              status: 'running',
              tool: 'list_available_values',
              params: { dimension: 'technology' },
            }),
            JSON.stringify({
              type: 'operation',
              status: 'completed',
              tool: 'list_available_values',
              params: { dimension: 'technology' },
            }),
            JSON.stringify({
              type: 'operation',
              status: 'running',
              tool: 'explain_draft',
            }),
            JSON.stringify({
              type: 'operation',
              status: 'completed',
              tool: 'explain_draft',
            }),
            JSON.stringify({
              type: 'draft',
              status: 'completed',
              draft: {
                contract_version: 'ai-trend-draft-v1',
                status: 'complete',
                curves: [
                  {
                    condition: {
                      dimension: 'technology',
                      value: 'redis',
                      includeSecondary: false,
                      operator: '',
                    },
                    label: 'Redis',
                    match_status: 'exact',
                    reason: '按当前正式值匹配',
                    selected: true,
                    sub_query_ids: ['technologies'],
                  },
                ],
                explanation: '按当前周期岗位平均数量整理候选，应用前仍可修改。',
              },
            }),
            JSON.stringify({ type: 'done', status: 'success' }),
          ].join('\n') + '\n',
          { headers: { 'Content-Type': 'application/x-ndjson' } },
        ),
      ),
    )
    const state = createAITrendDraftState()

    await state.submit('哪些数据库正在被使用？')

    expect(state.status).toBe('success')
    expect(state.draft?.curves).toHaveLength(1)
    expect(state.draft?.explanation).toContain('岗位平均数量')
    expect(state.operations.map((operation) => operation.tool)).toContain('analyze_intent')
    expect(state.operations.map((operation) => operation.tool)).toContain('explain_draft')
    expect(state.operations.map((operation) => operation.tool)).toContain('list_available_values')
  })

  it('clears the previous pending draft when regenerating', async () => {
    let fetchCount = 0
    let releaseSecondRequest: (() => void) | undefined
    const draftEvent = JSON.stringify({
      type: 'draft',
      status: 'completed',
      draft: {
        contract_version: 'ai-trend-draft-v1',
        status: 'complete',
        curves: [
          {
            condition: { dimension: 'project', value: 'alpha', includeSecondary: false, operator: '' },
            label: 'Alpha',
            match_status: 'exact',
            reason: '正式项目',
            selected: true,
            sub_query_ids: ['projects'],
          },
        ],
      },
    })
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        fetchCount += 1
        if (fetchCount === 1) {
          return new Response(`${draftEvent}\n${JSON.stringify({ type: 'done', status: 'success' })}\n`)
        }
        await new Promise<void>((resolve) => {
          releaseSecondRequest = resolve
        })
        return new Response(`${draftEvent}\n${JSON.stringify({ type: 'done', status: 'success' })}\n`)
      }),
    )
    const state = createAITrendDraftState()
    await state.submit('哪些项目')
    expect(state.draft?.curves).toHaveLength(1)

    const regeneration = state.submit('哪些技术')
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(state.status).toBe('running')
    expect(state.draft).toBeUndefined()
    expect(state.assistantMessage).toBe('')
    releaseSecondRequest?.()
    await regeneration
    expect(state.draft?.curves).toHaveLength(1)
  })

  it('supports a successful guidance response without a draft', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        new Response(
          [
            JSON.stringify({ type: 'assistant_message', status: 'running', delta: '你好，' }),
            JSON.stringify({ type: 'assistant_message', status: 'running', delta: '可以查看趋势。' }),
            JSON.stringify({ type: 'assistant_message', status: 'completed' }),
            JSON.stringify({ type: 'done', status: 'success' }),
          ].join('\n') + '\n',
          { headers: { 'Content-Type': 'application/x-ndjson' } },
        ),
      ),
    )
    const state = createAITrendDraftState()

    await state.submit('你好')

    expect(state.status).toBe('success')
    expect(state.assistantMessage).toBe('你好，可以查看趋势。')
    expect(state.draft).toBeUndefined()
  })
})
