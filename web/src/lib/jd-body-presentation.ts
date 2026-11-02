export type JDSectionItem =
  | { end: number; kind: 'lead'; start: number; text: string }
  | { end: number; kind: 'ordered-item'; marker: string; start: number; text: string }
  | { end: number; kind: 'bullet-item'; marker: string; start: number; text: string }

export type JDSectionPresentation =
  | { items: JDSectionItem[]; kind: 'list' }
  | { end: number; kind: 'fallback'; start: number; text: string }

type Marker = {
  endUTF16: number
  kind: 'ordered-item' | 'bullet-item'
  marker: string
  serial?: number
  startUTF16: number
}

const chineseOrdinalValues: Record<string, number> = {
  一: 1,
  七: 7,
  三: 3,
  九: 9,
  二: 2,
  五: 5,
  八: 8,
  六: 6,
  十: 10,
  四: 4,
}

export function presentJDSection(text: string): JDSectionPresentation {
  const ordinalMarkers = collectOrdinalMarkers(text)
  if (hasConsecutiveSerials(ordinalMarkers)) {
    const list = buildList(text, ordinalMarkers)
    if (list) return list
  }

  const bulletMarkers = collectBulletMarkers(text)
  if (bulletMarkers.length >= 2) {
    const list = buildList(text, bulletMarkers)
    if (list) return list
  }

  return { end: codePointLength(text), kind: 'fallback', start: 0, text }
}

function collectOrdinalMarkers(text: string): Marker[] {
  const markers: Marker[] = []
  const numeric = /(?:^|[\s；;])([1-9]\d?[、.．）)])(?=[^\d])/gu
  for (const match of text.matchAll(numeric)) {
    const marker = match[1] ?? ''
    markers.push(markerFromMatch(match, marker, 'ordered-item', Number.parseInt(marker, 10)))
  }

  const chinese = /(?:^|[\s；;])([一二三四五六七八九十][、.．）)])/gu
  for (const match of text.matchAll(chinese)) {
    const marker = match[1] ?? ''
    markers.push(
      markerFromMatch(match, marker, 'ordered-item', chineseOrdinalValues[marker[0] ?? '']),
    )
  }
  return markers.sort((left, right) => left.startUTF16 - right.startUTF16)
}

function collectBulletMarkers(text: string): Marker[] {
  const markers: Marker[] = []
  const bullet = /(?:^|[\s；;])([-*•▪])[ \t]+/gu
  for (const match of text.matchAll(bullet)) {
    const marker = match[1] ?? ''
    markers.push(markerFromMatch(match, marker, 'bullet-item'))
  }
  return markers
}

function markerFromMatch(
  match: RegExpMatchArray,
  marker: string,
  kind: Marker['kind'],
  serial?: number,
): Marker {
  const fullMatch = match[0] ?? ''
  const startUTF16 = (match.index ?? 0) + fullMatch.lastIndexOf(marker)
  return {
    endUTF16: startUTF16 + marker.length,
    kind,
    marker,
    serial,
    startUTF16,
  }
}

function hasConsecutiveSerials(markers: Marker[]): boolean {
  return (
    markers.length >= 2 &&
    markers.every(
      (marker, index) =>
        marker.serial !== undefined &&
        (index === 0 || marker.serial === (markers[index - 1]?.serial ?? 0) + 1),
    )
  )
}

function buildList(
  text: string,
  markers: Marker[],
): Extract<JDSectionPresentation, { kind: 'list' }> | undefined {
  const items: JDSectionItem[] = []
  const leading = trimmedSegment(text, 0, markers[0]?.startUTF16 ?? 0)
  if (leading) {
    items.push({
      end: codePointLength(text.slice(0, leading.endUTF16)),
      kind: 'lead',
      start: codePointLength(text.slice(0, leading.startUTF16)),
      text: leading.text,
    })
  }

  for (const [index, marker] of markers.entries()) {
    const next = markers[index + 1]
    const item = trimmedSegment(text, marker.endUTF16, next?.startUTF16 ?? text.length)
    if (!item) return undefined
    items.push({
      end: codePointLength(text.slice(0, item.endUTF16)),
      kind: marker.kind,
      marker: marker.marker,
      start: codePointLength(text.slice(0, item.startUTF16)),
      text: item.text,
    })
  }
  return { items, kind: 'list' }
}

function trimmedSegment(
  text: string,
  startUTF16: number,
  endUTF16: number,
): { endUTF16: number; startUTF16: number; text: string } | undefined {
  const source = text.slice(startUTF16, endUTF16)
  const trimmed = source.trim()
  if (!trimmed) return undefined
  const leadingWhitespace = source.length - source.trimStart().length
  return {
    endUTF16: startUTF16 + leadingWhitespace + trimmed.length,
    startUTF16: startUTF16 + leadingWhitespace,
    text: trimmed,
  }
}

function codePointLength(text: string): number {
  return Array.from(text).length
}
