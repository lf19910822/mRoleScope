export type JDDetailBodySection = 'responsibilities' | 'requirements' | 'supplemental'

export type TechnologyHighlightRange = {
  end: number
  section: JDDetailBodySection
  start: number
}

export type HighlightedTextFragment = {
  highlighted: boolean
  text: string
}

// fragmentHighlightedText turns display-safe, section-relative Unicode
// code-point ranges into safe text fragments. It deliberately does no keyword
// matching: only the serving layer may decide what counts as a technology hit.
export function fragmentHighlightedText(
  sourceText: string,
  blockStart: number,
  blockEnd: number,
  section: JDDetailBodySection,
  ranges: readonly TechnologyHighlightRange[] | undefined,
): HighlightedTextFragment[] {
  const codePoints = Array.from(sourceText)
  if (!isValidBlock(blockStart, blockEnd, codePoints.length)) {
    return [{ highlighted: false, text: sourceText }]
  }

  const intervals = mergeIntervals(
    (ranges ?? [])
      .filter(
        (range) =>
          range.section === section &&
          isValidBlock(range.start, range.end, codePoints.length) &&
          range.end > blockStart &&
          range.start < blockEnd,
      )
      .map((range) => ({
        end: Math.min(range.end, blockEnd),
        start: Math.max(range.start, blockStart),
      })),
  )

  if (intervals.length === 0) {
    return [{ highlighted: false, text: codePoints.slice(blockStart, blockEnd).join('') }]
  }

  const fragments: HighlightedTextFragment[] = []
  let cursor = blockStart
  for (const interval of intervals) {
    if (cursor < interval.start) {
      fragments.push({
        highlighted: false,
        text: codePoints.slice(cursor, interval.start).join(''),
      })
    }
    fragments.push({
      highlighted: true,
      text: codePoints.slice(interval.start, interval.end).join(''),
    })
    cursor = interval.end
  }
  if (cursor < blockEnd) {
    fragments.push({ highlighted: false, text: codePoints.slice(cursor, blockEnd).join('') })
  }
  return fragments
}

function isValidBlock(start: number, end: number, length: number): boolean {
  return (
    Number.isInteger(start) && Number.isInteger(end) && start >= 0 && end > start && end <= length
  )
}

function mergeIntervals(
  intervals: Array<{ end: number; start: number }>,
): Array<{ end: number; start: number }> {
  const ordered = intervals.sort((left, right) => left.start - right.start || left.end - right.end)
  const merged: Array<{ end: number; start: number }> = []
  for (const interval of ordered) {
    const previous = merged[merged.length - 1]
    if (previous && interval.start <= previous.end) {
      previous.end = Math.max(previous.end, interval.end)
      continue
    }
    merged.push({ ...interval })
  }
  return merged
}
