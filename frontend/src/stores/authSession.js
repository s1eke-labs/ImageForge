const tokenKey = 'imageforge_token'
const usernameKey = 'imageforge_username'

export function getSession() {
  return {
    token: window.localStorage.getItem(tokenKey) || '',
    username: window.localStorage.getItem(usernameKey) || '',
  }
}

export function getToken() {
  return window.localStorage.getItem(tokenKey) || ''
}

export function setSession(token, username) {
  window.localStorage.setItem(tokenKey, token)
  window.localStorage.setItem(usernameKey, username)
}

export function clearSession() {
  window.localStorage.removeItem(tokenKey)
  window.localStorage.removeItem(usernameKey)
}
