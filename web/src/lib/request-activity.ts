import { readonly, ref, type Ref } from 'vue'

export type RequestActivityCoordinator = {
  activeCount: Readonly<Ref<number>>
  begin: () => () => void
  visible: Readonly<Ref<boolean>>
}

// A token-based coordinator makes overlapping list, detail, and vocabulary
// reads one application-level activity without allowing a fast request to
// hide an unrelated slow request.
export function createRequestActivityCoordinator(
  delayMilliseconds = 180,
): RequestActivityCoordinator {
  const activeCount = ref(0)
  const visible = ref(false)
  let revealTimer: ReturnType<typeof setTimeout> | undefined

  function clearRevealTimer(): void {
    if (revealTimer !== undefined) {
      clearTimeout(revealTimer)
      revealTimer = undefined
    }
  }

  function begin(): () => void {
    activeCount.value += 1
    if (activeCount.value === 1) {
      clearRevealTimer()
      revealTimer = setTimeout(() => {
        revealTimer = undefined
        if (activeCount.value > 0) {
          visible.value = true
        }
      }, delayMilliseconds)
    }

    let settled = false
    return () => {
      if (settled) {
        return
      }
      settled = true
      activeCount.value = Math.max(0, activeCount.value - 1)
      if (activeCount.value === 0) {
        clearRevealTimer()
        visible.value = false
      }
    }
  }

  return {
    activeCount: readonly(activeCount),
    begin,
    visible: readonly(visible),
  }
}

export const requestActivity = createRequestActivityCoordinator()
