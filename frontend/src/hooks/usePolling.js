import { useEffect, useRef } from 'react'

export function usePolling(callback, intervalMs, enabled) {
  const callbackRef = useRef(callback)

  useEffect(() => {
    callbackRef.current = callback
  }, [callback])

  useEffect(() => {
    if (!enabled) return undefined

    let intervalId
    const run = () => {
      if (document.visibilityState === 'visible') {
        callbackRef.current()
      }
    }
    const start = () => {
      clearInterval(intervalId)
      intervalId = window.setInterval(run, intervalMs)
    }
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        run()
        start()
      } else {
        clearInterval(intervalId)
      }
    }

    run()
    start()
    document.addEventListener('visibilitychange', handleVisibility)

    return () => {
      clearInterval(intervalId)
      document.removeEventListener('visibilitychange', handleVisibility)
    }
  }, [enabled, intervalMs])
}
