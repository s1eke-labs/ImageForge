import { useMemo, useState } from 'react'
import { defaultLocale, messages, supportedLocales } from './messages.js'
import { I18nContext } from './useI18n.js'

const storageKey = 'imageforge_locale'

function normalizeLocale(value) {
  if (!value) return ''
  const exact = supportedLocales.find((locale) => locale.toLowerCase() === value.toLowerCase())
  if (exact) return exact
  const language = value.toLowerCase().split('-')[0]
  return supportedLocales.find((locale) => locale.toLowerCase().startsWith(`${language}-`)) || ''
}

function initialLocale() {
  const stored = normalizeLocale(window.localStorage.getItem(storageKey))
  if (stored) return stored
  const detected = normalizeLocale(window.navigator.language)
  return detected || defaultLocale
}

function interpolate(template, values) {
  if (!values) return template
  return template.replace(/\{(\w+)\}/g, (_match, key) => String(values[key] ?? ''))
}

export default function I18nProvider({ children }) {
  const [locale, setLocaleState] = useState(initialLocale)

  const value = useMemo(() => {
    const setLocale = (nextLocale) => {
      const normalized = normalizeLocale(nextLocale) || defaultLocale
      window.localStorage.setItem(storageKey, normalized)
      setLocaleState(normalized)
    }
    const t = (key, values) => {
      const template = messages[locale]?.[key] ?? messages[defaultLocale]?.[key] ?? key
      return interpolate(template, values)
    }
    return { locale, setLocale, t }
  }, [locale])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}
