import { useCallback, useSyncExternalStore } from 'react'

function getIsDark() {
  return document.documentElement.classList.contains('dark')
}

const listeners = new Set<() => void>()
function subscribe(cb: () => void) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

export function useTheme() {
  const isDark = useSyncExternalStore(subscribe, getIsDark)

  const toggle = useCallback(() => {
    const next = !document.documentElement.classList.contains('dark')
    document.documentElement.classList.toggle('dark', next)
    localStorage.setItem('darkTheme', String(next))
    listeners.forEach((cb) => cb())
  }, [])

  return { isDark, toggle }
}
