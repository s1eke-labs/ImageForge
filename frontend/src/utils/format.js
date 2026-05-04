export function relativeTime(value) {
  if (!value) return ''
  const date = new Date(value)
  const seconds = Math.max(1, Math.round((Date.now() - date.getTime()) / 1000))
  if (seconds < 60) return `${seconds} 秒前`
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.round(hours / 24)
  return `${days} 天前`
}

export function bytes(value) {
  if (!value) return '-'
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(2)} MB`
}

export function dateTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}
