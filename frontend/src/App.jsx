import { Navigate, Route, Routes, useLocation } from 'react-router'
import LoginPage from './pages/LoginPage.jsx'
import CreatePage from './pages/CreatePage.jsx'
import ArtworkPage from './pages/ArtworkPage.jsx'
import { useAuthStore } from './stores/authStore.js'

function RequireAuth({ children }) {
  const token = useAuthStore((state) => state.token)
  const location = useLocation()
  if (!token) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }
  return children
}

function AppShell({ children }) {
  return (
    <main className="app-shell">
      <section className="app-content">{children}</section>
    </main>
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Navigate to="/create" replace />} />
      <Route
        path="/create"
        element={
          <RequireAuth>
            <AppShell>
              <CreatePage />
            </AppShell>
          </RequireAuth>
        }
      />
      <Route
        path="/artworks/:id"
        element={
          <RequireAuth>
            <ArtworkPage />
          </RequireAuth>
        }
      />
      <Route path="*" element={<Navigate to="/create" replace />} />
    </Routes>
  )
}
