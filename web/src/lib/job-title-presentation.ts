export type JobTitleFragment = {
  kind: 'project' | 'text'
  text: string
}

type ProjectLabel = {
  label: string
  value?: string
}

export function presentJobTitle(
  title: string,
  projects: readonly ProjectLabel[],
): JobTitleFragment[] {
  const labels = [...new Set(projects.map((project) => project.label.trim()))]
    .filter((label) => label.length > 0 && title.includes(label))
    .sort((left, right) => right.length - left.length || left.localeCompare(right))

  if (labels.length === 0) {
    return [{ kind: 'text', text: title }]
  }

  const fragments: JobTitleFragment[] = []
  let cursor = 0

  while (cursor < title.length) {
    let nextIndex = -1
    let nextLabel: string | undefined

    for (const label of labels) {
      const index = title.indexOf(label, cursor)
      if (
        index !== -1 &&
        (nextIndex === -1 ||
          index < nextIndex ||
          (index === nextIndex && label.length > nextLabel!.length))
      ) {
        nextIndex = index
        nextLabel = label
      }
    }

    if (nextLabel === undefined || nextIndex === -1) {
      fragments.push({ kind: 'text', text: title.slice(cursor) })
      break
    }

    if (nextIndex > cursor) {
      fragments.push({ kind: 'text', text: title.slice(cursor, nextIndex) })
    }
    fragments.push({ kind: 'project', text: nextLabel })
    cursor = nextIndex + nextLabel.length
  }

  return fragments
}
