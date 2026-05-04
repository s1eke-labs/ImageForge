import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { LogIn } from 'lucide-react'
import LanguageSwitcher from '../components/LanguageSwitcher.jsx'
import logo from '../assets/logo.svg'
import { useI18n } from '../i18n/useI18n.js'
import { useAuthStore } from '../stores/authStore.js'

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const login = useAuthStore((state) => state.login)
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useI18n()

  const submit = async (event) => {
    event.preventDefault()
    setLoading(true)
    setError('')
    try {
      await login(username, password)
      navigate(location.state?.from?.pathname || '/create', { replace: true })
    } catch {
      setError(t('login.error'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-page">
      <form className="login-panel" onSubmit={submit}>
        <LanguageSwitcher className="login-language-switcher" />
        <div className="login-logo" aria-hidden="true">
          <img src={logo} alt="" />
        </div>
        <div className="login-copy">
          <h1>{t('login.title')}</h1>
          <p>{t('login.subtitle')}</p>
        </div>
        <label>
          {t('login.username')}
          <input
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            autoComplete="username"
            placeholder={t('login.usernamePlaceholder')}
            required
          />
        </label>
        <label>
          {t('login.password')}
          <input
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            type="password"
            autoComplete="current-password"
            placeholder={t('login.passwordPlaceholder')}
            required
          />
        </label>
        {error && <div className="form-error">{error}</div>}
        <button className="primary-button" type="submit" disabled={loading}>
          <LogIn size={18} />
          {loading ? t('login.loading') : t('login.submit')}
        </button>
      </form>
    </main>
  )
}
