import { useCallback, useEffect } from 'react'
import { AlertCircle, CheckCircle2, LoaderCircle, X } from 'lucide-react'
import TaskCard from '../components/TaskCard.jsx'
import { usePolling } from '../hooks/usePolling.js'
import { useTaskStore } from '../stores/taskStore.js'

export default function TaskListPage({ open = true, onClose }) {
  const tasks = useTaskStore((state) => state.tasks)
  const loading = useTaskStore((state) => state.loading)
  const fetchTasks = useTaskStore((state) => state.fetchTasks)
  const refresh = useCallback(() => {
    fetchTasks().catch(() => {})
  }, [fetchTasks])
  const hasActiveTasks = tasks.some((task) => task.status === 'pending' || task.status === 'claimed')

  useEffect(() => {
    refresh()
  }, [refresh])
  usePolling(refresh, 5000, hasActiveTasks)

  const activeTasks = tasks.filter((task) => task.status === 'pending' || task.status === 'claimed')
  const completedTasks = tasks.filter((task) => task.status === 'succeeded')
  const failedTasks = tasks.filter((task) => task.status === 'failed' || task.status === 'canceled')

  return (
    <aside className={`tasks-drawer ${open ? 'open' : ''}`} aria-hidden={!open}>
      <header className="tasks-header">
        <h1>Generation Tasks</h1>
        <button type="button" className="topbar-icon" onClick={onClose} aria-label="Close generation tasks">
          <X size={28} />
        </button>
      </header>
      {!loading && tasks.length === 0 && <div className="empty-state">No generation tasks yet.</div>}
      <section className="task-section">
        <h2>
          <LoaderCircle size={18} />
          In Progress
        </h2>
        <div className="task-list">{activeTasks.map((task) => <TaskCard key={task.id} task={task} />)}</div>
      </section>
      <section className="task-section">
        <h2>
          <CheckCircle2 size={18} />
          Completed
        </h2>
        <div className="task-list">{completedTasks.map((task) => <TaskCard key={task.id} task={task} />)}</div>
      </section>
      <section className="task-section">
        <h2>
          <AlertCircle size={18} />
          Failed
        </h2>
        <div className="task-list">{failedTasks.map((task) => <TaskCard key={task.id} task={task} />)}</div>
      </section>
    </aside>
  )
}
