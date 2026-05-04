import { create } from 'zustand'
import { api } from '../api/client.js'

export const useTaskStore = create((set, get) => ({
  tasks: [],
  total: 0,
  page: 1,
  pageSize: 20,
  loading: false,
  async fetchTasks() {
    set({ loading: true })
    try {
      const data = await api.get('tasks', { searchParams: { page: get().page, page_size: get().pageSize } }).json()
      set({ tasks: data.tasks || [], total: data.total || 0, loading: false })
    } catch (error) {
      set({ loading: false })
      throw error
    }
  },
  async createTask(formData) {
    return api.post('tasks', { body: formData }).json()
  },
  async retryTask(id) {
    return api.post(`tasks/${id}/retry`).json()
  },
  async getTask(id) {
    return api.get(`tasks/${id}`).json()
  },
}))
