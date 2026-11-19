import { describe, expect, it } from 'vitest'

import { createAITrendDraftState } from '../ai-trend-draft-state'

describe('AI trend draft state', () => {
  it('provides a local mock draft with operation history for development preview', async () => {
    const state = createAITrendDraftState()

    await state.submit('哪些数据库正在被使用？')

    expect(state.status).toBe('success')
    expect(state.draft?.curves.length).toBeGreaterThanOrEqual(4)
    expect(state.draft?.explanation).toContain('岗位平均数量')
    expect(state.operations.map((operation) => operation.tool)).toContain('batch_trend_summary')
  })
})
