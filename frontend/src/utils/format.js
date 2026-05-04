export function relativeTime(value, locale = 'zh-CN') {
  if (!value) return ''
  const date = new Date(value)
  const seconds = Math.max(1, Math.round((Date.now() - date.getTime()) / 1000))
  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'always' })
  if (seconds < 60) return formatter.format(-seconds, 'second')
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return formatter.format(-minutes, 'minute')
  const hours = Math.round(minutes / 60)
  if (hours < 24) return formatter.format(-hours, 'hour')
  const days = Math.round(hours / 24)
  return formatter.format(-days, 'day')
}

export function duration(value, locale = 'zh-CN') {
  if (!value) return '-'
  const seconds = Math.max(1, Math.round(value))
  const zh = locale === 'zh-CN'
  if (seconds < 60) return zh ? `${seconds} 秒` : `${seconds} sec`
  const minutes = Math.floor(seconds / 60)
  const rest = Math.round(seconds % 60)
  if (minutes < 60) {
    if (!rest) return zh ? `${minutes} 分` : `${minutes} min`
    return zh ? `${minutes} 分 ${rest} 秒` : `${minutes} min ${rest} sec`
  }
  const hours = Math.floor(minutes / 60)
  const minuteRest = minutes % 60
  if (!minuteRest) return zh ? `${hours} 小时` : `${hours} hr`
  return zh ? `${hours} 小时 ${minuteRest} 分` : `${hours} hr ${minuteRest} min`
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
