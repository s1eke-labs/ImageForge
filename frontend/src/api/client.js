import ky from 'ky'
import { useAuthStore } from '../stores/authStore.js'
import { getToken } from '../stores/authSession.js'

export const api = ky.create({
  prefixUrl: `${window.location.origin}/api`,
  hooks: {
    beforeRequest: [
      (request) => {
        const token = getToken()
        if (token) {
          request.headers.set('Authorization', `Bearer ${token}`)
        }
      },
    ],
    afterResponse: [
      (_request, _options, response) => {
        if (response.status === 401) {
          useAuthStore.getState().logout()
        }
      },
    ],
  },
})

export function getFileBlob(path) {
  return api.get(`files/${path}`).blob()
}
