import { create } from 'zustand'
import { api } from '../api/client.js'

function mergeTaskStatusUpdates(tasks, updates) {
  const updatesById = new Map(updates.map((task) => [task.id, task]))
  return tasks.map((task) => {
    const update = updatesById.get(task.id)
    return update ? { ...task, ...update } : task
  })
}

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
  async fetchTaskStatuses(ids) {
    const taskIds = Array.from(new Set((ids || []).filter(Boolean)))
    if (taskIds.length === 0) return []
    const data = await api.get('tasks/statuses', { searchParams: { ids: taskIds.join(',') } }).json()
    const updates = data.tasks || []
    if (updates.length > 0) {
      set((state) => ({ tasks: mergeTaskStatusUpdates(state.tasks, updates) }))
    }
    return updates
  },
  async createTask(formData) {
    const task = await api.post('tasks', { body: formData }).json()
    set((state) => ({
      tasks: [task, ...state.tasks.filter((existing) => existing.id !== task.id)].slice(0, state.pageSize),
      total: state.total + 1,
    }))
    return task
  },
  async retryTask(id) {
    return api.post(`tasks/${id}/retry`).json()
  },
  async getTask(id) {
    return api.get(`tasks/${id}`).json()
  },
}))
