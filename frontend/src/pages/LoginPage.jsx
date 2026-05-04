import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { LogIn } from 'lucide-react'
import logo from '../assets/logo.svg'
import { useAuthStore } from '../stores/authStore.js'

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const login = useAuthStore((state) => state.login)
  const navigate = useNavigate()
  const location = useLocation()

  const submit = async (event) => {
    event.preventDefault()
    setLoading(true)
    setError('')
    try {
      await login(username, password)
      navigate(location.state?.from?.pathname || '/create', { replace: true })
    } catch {
      setError('Username or password is incorrect')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="login-page">
      <form className="login-panel" onSubmit={submit}>
        <div className="login-logo" aria-hidden="true">
          <img src={logo} alt="" />
        </div>
        <div className="login-copy">
          <h1>Welcome Back</h1>
          <p>Sign in to continue.</p>
        </div>
        <label>
          Username
          <input
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            autoComplete="username"
            placeholder="Enter your username"
            required
          />
        </label>
        <label>
          Password
          <input
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            type="password"
            autoComplete="current-password"
            placeholder="Enter your password"
            required
          />
        </label>
        {error && <div className="form-error">{error}</div>}
        <button className="primary-button" type="submit" disabled={loading}>
          <LogIn size={18} />
          {loading ? 'Logging in' : 'Log In'}
        </button>
      </form>
    </main>
  )
}
