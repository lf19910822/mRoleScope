export function localAccessEntryPath(url: string | undefined): string | undefined {
  if (!url) return undefined

  const queryIndex = url.indexOf('?')
  const pathname = queryIndex === -1 ? url : url.slice(0, queryIndex)
  if (pathname !== '/access' && pathname !== '/access/') return undefined

  return queryIndex === -1 ? '/access/index.html' : `/access/index.html${url.slice(queryIndex)}`
}
