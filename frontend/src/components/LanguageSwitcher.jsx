import { Languages } from 'lucide-react'
import { localeNames, supportedLocales } from '../i18n/messages.js'
import { useI18n } from '../i18n/useI18n.js'

export default function LanguageSwitcher({ className = '' }) {
  const { locale, setLocale, t } = useI18n()
  const nextLocale = supportedLocales.find((item) => item !== locale) || supportedLocales[0]

  return (
    <button
      type="button"
      className={`language-switcher ${className}`.trim()}
      onClick={() => setLocale(nextLocale)}
      aria-label={t('app.language')}
      title={t('app.language')}
    >
      <Languages size={17} aria-hidden="true" />
      <span>{localeNames[locale]}</span>
    </button>
  )
}
