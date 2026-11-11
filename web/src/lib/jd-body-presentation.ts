export type JDSectionChildItem = {
  end: number
  kind: 'bullet-item'
  marker: string
  start: number
  text: string
}

export type JDSectionItem =
  | { end: number; kind: 'lead'; start: number; text: string }
  | { end: number; kind: 'group-heading'; start: number; text: string }
  | {
      childItems?: JDSectionChildItem[]
      childLeadEnd?: number
      end: number
      kind: 'ordered-item'
      marker: string
      start: number
      text: string
    }
  | {
      childItems?: never
      childLeadEnd?: never
      end: number
      kind: 'bullet-item'
      marker: string
      start: number
      text: string
    }

export type JDSectionPresentation =
  | { items: JDSectionItem[]; kind: 'list' }
  | { end: number; kind: 'fallback'; start: number; text: string }

type Marker = {
  ambiguous?: boolean
  endUTF16: number
  kind: 'ordered-item' | 'bullet-item'
  marker: string
  ordinalStyle?: OrdinalStyle
  serial?: number
  startUTF16: number
  styleGapAllowed?: boolean
}

type OrdinalStyle =
  | 'arabic-comma'
  | 'arabic-dot'
  | 'arabic-parenthesis'
  | 'chinese-comma'
  | 'chinese-dot'
  | 'chinese-parenthesis'

type GroupHeading = {
  endUTF16: number
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
  const ordinalList = buildOrdinalList(text, ordinalMarkers)
  if (ordinalList) return ordinalList

  const bulletMarkers = collectBulletMarkers(text)
  if (bulletMarkers.length >= 2) {
    const list = buildList(text, bulletMarkers)
    if (list) return list
  }

  return { end: codePointLength(text), kind: 'fallback', start: 0, text }
}

function buildOrdinalList(
  text: string,
  markers: Marker[],
): Extract<JDSectionPresentation, { kind: 'list' }> | undefined {
  markers = selectEstablishedOrdinalStyle(markers)
  if (
    markers.length === 0 ||
    markers.some((marker) => marker.serial === undefined) ||
    !hasOnlyInteriorAmbiguousMarkerRuns(markers)
  ) {
    return undefined
  }

  const initialGroupHeading = trailingGroupHeading(text, 0, markers[0]?.startUTF16 ?? 0)
  const groupHeadings = new Map<number, GroupHeading>()
  let explicitGroupAllowsGaps = initialGroupHeading !== undefined && markers[0]?.serial === 1

  for (let index = 1; index < markers.length; index += 1) {
    const marker = markers[index]
    const previous = markers[index - 1]
    if (!marker || !previous || marker.serial === undefined || previous.serial === undefined) {
      return undefined
    }
    if (marker.serial === previous.serial + 1) continue

    if (marker.serial !== 1) {
      if ((explicitGroupAllowsGaps || marker.styleGapAllowed) && marker.serial > previous.serial) {
        continue
      }
      return undefined
    }

    const heading = trailingGroupHeading(text, previous.endUTF16, marker.startUTF16)
    if (!heading) return undefined
    groupHeadings.set(index, heading)
    explicitGroupAllowsGaps = true
  }

  return buildList(text, markers, groupHeadings, initialGroupHeading)
}

function collectOrdinalMarkers(text: string): Marker[] {
  const markers: Marker[] = []
  const numeric = /(?:^|[\s；;])([1-9]\d?[、）)]|[1-9]\d?[ \t]?[.．])/gu
  for (const match of text.matchAll(numeric)) {
    const marker = match[1] ?? ''
    markers.push(
      markerFromMatch(
        match,
        marker,
        'ordered-item',
        Number.parseInt(marker, 10),
        isAmbiguousNumericDotMarker(text, match, marker),
        ordinalStyle(marker),
      ),
    )
  }

  const chinese = /(?:^|[\s；;])([一二三四五六七八九十][、.．）)])/gu
  for (const match of text.matchAll(chinese)) {
    const marker = match[1] ?? ''
    markers.push(
      markerFromMatch(
        match,
        marker,
        'ordered-item',
        chineseOrdinalValues[marker[0] ?? ''],
        false,
        ordinalStyle(marker),
      ),
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
  ambiguous = false,
  ordinalStyle?: OrdinalStyle,
): Marker {
  const fullMatch = match[0] ?? ''
  const startUTF16 = (match.index ?? 0) + fullMatch.lastIndexOf(marker)
  return {
    ambiguous,
    endUTF16: startUTF16 + marker.length,
    kind,
    marker,
    ordinalStyle,
    serial,
    startUTF16,
  }
}

function ordinalStyle(marker: string): OrdinalStyle {
  const isChinese = /^[一二三四五六七八九十]/u.test(marker)
  const punctuation = marker.slice(-1)
  if (punctuation === '、') return isChinese ? 'chinese-comma' : 'arabic-comma'
  if (punctuation === ')' || punctuation === '）') {
    return isChinese ? 'chinese-parenthesis' : 'arabic-parenthesis'
  }
  return isChinese ? 'chinese-dot' : 'arabic-dot'
}

function selectEstablishedOrdinalStyle(markers: Marker[]): Marker[] {
  const pair = markers.find((marker, index) => {
    const next = markers[index + 1]
    return Boolean(
      next &&
      !marker.ambiguous &&
      !next.ambiguous &&
      marker.serial !== undefined &&
      next.serial === marker.serial + 1 &&
      marker.ordinalStyle !== undefined &&
      marker.ordinalStyle === next.ordinalStyle,
    )
  })
  if (!pair?.ordinalStyle) return markers

  const selected: Marker[] = []
  const ignoredSerials: number[] = []
  for (const marker of markers) {
    if (marker.ordinalStyle !== pair.ordinalStyle) {
      if (marker.serial !== undefined) ignoredSerials.push(marker.serial)
      continue
    }

    const previous = selected[selected.length - 1]
    if (
      previous?.serial !== undefined &&
      marker.serial !== undefined &&
      marker.serial > previous.serial + 1
    ) {
      marker.styleGapAllowed = hasCompleteIgnoredBridge(
        previous.serial,
        marker.serial,
        ignoredSerials,
      )
    }
    selected.push(marker)
    ignoredSerials.length = 0
  }
  return selected
}

function hasCompleteIgnoredBridge(
  previousSerial: number,
  serial: number,
  ignoredSerials: number[],
) {
  const ignored = new Set(ignoredSerials)
  for (let expected = previousSerial + 1; expected < serial; expected += 1) {
    if (!ignored.has(expected)) return false
  }
  return serial > previousSerial + 1
}

function isAmbiguousNumericDotMarker(
  text: string,
  match: RegExpMatchArray,
  marker: string,
): boolean {
  if (!/[.．]$/u.test(marker)) return false

  const fullMatch = match[0] ?? ''
  const markerStartUTF16 = (match.index ?? 0) + fullMatch.lastIndexOf(marker)
  return /^\d$/u.test(text[markerStartUTF16 + marker.length] ?? '')
}

function hasOnlyInteriorAmbiguousMarkerRuns(markers: Marker[]): boolean {
  let index = 0
  while (index < markers.length) {
    if (!markers[index]?.ambiguous) {
      index += 1
      continue
    }

    const runStart = index
    while (markers[index]?.ambiguous) index += 1

    if (runStart === 0 || index === markers.length) return false
  }
  return true
}

function buildList(
  text: string,
  markers: Marker[],
  groupHeadings = new Map<number, GroupHeading>(),
  initialGroupHeading?: GroupHeading,
): Extract<JDSectionPresentation, { kind: 'list' }> | undefined {
  const items: JDSectionItem[] = []
  const leading = trimmedSegment(
    text,
    0,
    initialGroupHeading?.startUTF16 ?? markers[0]?.startUTF16 ?? 0,
  )
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
    const heading = index === 0 ? initialGroupHeading : groupHeadings.get(index)
    if (heading) {
      items.push({
        end: codePointLength(text.slice(0, heading.endUTF16)),
        kind: 'group-heading',
        start: codePointLength(text.slice(0, heading.startUTF16)),
        text: text.slice(heading.startUTF16, heading.endUTF16),
      })
    }

    const nextHeading = groupHeadings.get(index + 1)
    const item = trimmedSegment(
      text,
      marker.endUTF16,
      nextHeading?.startUTF16 ?? next?.startUTF16 ?? text.length,
    )
    if (!item) return undefined
    items.push(listItemFromSegment(text, marker, item))
  }
  return { items, kind: 'list' }
}

function listItemFromSegment(
  text: string,
  marker: Marker,
  item: { endUTF16: number; startUTF16: number; text: string },
): Extract<JDSectionItem, { kind: 'bullet-item' } | { kind: 'ordered-item' }> {
  const base = {
    end: codePointLength(text.slice(0, item.endUTF16)),
    marker: marker.marker,
    start: codePointLength(text.slice(0, item.startUTF16)),
    text: item.text,
  }
  if (marker.kind === 'bullet-item') return { ...base, kind: marker.kind }

  const childList = compactChildList(text, item.startUTF16, item.endUTF16)
  if (!childList) return { ...base, kind: marker.kind }

  return {
    ...base,
    childItems: childList.items,
    childLeadEnd: codePointLength(text.slice(0, childList.leadEndUTF16)),
    kind: marker.kind,
  }
}

function compactChildList(
  text: string,
  startUTF16: number,
  endUTF16: number,
): { items: JDSectionChildItem[]; leadEndUTF16: number } | undefined {
  const source = text.slice(startUTF16, endUTF16)
  const markers: Marker[] = []
  const compactHyphen = /(?:^|[\s；;])(-)(?=\S)/gu
  for (const match of source.matchAll(compactHyphen)) {
    const marker = match[1] ?? ''
    const fullMatch = match[0] ?? ''
    const markerStartUTF16 = startUTF16 + (match.index ?? 0) + fullMatch.lastIndexOf(marker)
    markers.push({
      endUTF16: markerStartUTF16 + marker.length,
      kind: 'bullet-item',
      marker,
      startUTF16: markerStartUTF16,
    })
  }
  if (markers.length < 2) return undefined

  const items: JDSectionChildItem[] = []
  for (const [index, marker] of markers.entries()) {
    const next = markers[index + 1]
    const item = trimmedSegment(text, marker.endUTF16, next?.startUTF16 ?? endUTF16)
    if (!item) return undefined
    items.push({
      end: codePointLength(text.slice(0, item.endUTF16)),
      kind: 'bullet-item',
      marker: marker.marker,
      start: codePointLength(text.slice(0, item.startUTF16)),
      text: item.text,
    })
  }

  return { items, leadEndUTF16: markers[0]?.startUTF16 ?? endUTF16 }
}

function trailingGroupHeading(
  text: string,
  startUTF16: number,
  endUTF16: number,
): GroupHeading | undefined {
  const source = text.slice(startUTF16, endUTF16)
  const match = /(?:^|[\s；;])((?:基础|加分项)[：:]?)\s*$/u.exec(source)
  const heading = match?.[1]
  if (!heading || match.index === undefined) return undefined

  const fullMatch = match[0] ?? ''
  const headingStartUTF16 = startUTF16 + match.index + fullMatch.lastIndexOf(heading)
  return {
    endUTF16: headingStartUTF16 + heading.length,
    startUTF16: headingStartUTF16,
  }
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
