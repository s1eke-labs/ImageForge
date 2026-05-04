import { useEffect, useState } from 'react'
import { getFileBlob } from '../api/client.js'

export function useFileObjectURL(path) {
  const [url, setUrl] = useState('')

  useEffect(() => {
    if (!path) {
      setUrl('')
      return undefined
    }

    let cancelled = false
    let objectURL = ''
    setUrl('')

    getFileBlob(path)
      .then((blob) => {
        if (cancelled) return
        objectURL = URL.createObjectURL(blob)
        setUrl(objectURL)
      })
      .catch(() => {})

    return () => {
      cancelled = true
      if (objectURL) URL.revokeObjectURL(objectURL)
    }
  }, [path])

  return url
}
