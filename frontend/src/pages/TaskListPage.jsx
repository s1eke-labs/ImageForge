import { useCallback, useEffect, useMemo } from 'react'
import { AlertCircle, CheckCircle2, LoaderCircle, X } from 'lucide-react'
import TaskCard from '../components/TaskCard.jsx'
import { usePolling } from '../hooks/usePolling.js'
import { useI18n } from '../i18n/useI18n.js'
import { useTaskStore } from '../stores/taskStore.js'

export default function TaskListPage({ open = true, onClose }) {
  const { t } = useI18n()
  const tasks = useTaskStore((state) => state.tasks)
  const loading = useTaskStore((state) => state.loading)
  const fetchTasks = useTaskStore((state) => state.fetchTasks)
  const fetchTaskStatuses = useTaskStore((state) => state.fetchTaskStatuses)
  const refresh = useCallback(() => {
    fetchTasks().catch(() => {})
  }, [fetchTasks])
  const activeTaskIds = useMemo(() => tasks.filter((task) => task.status === 'pending' || task.status === 'claimed').map((task) => task.id), [tasks])
  const refreshActiveStatuses = useCallback(() => {
    fetchTaskStatuses(activeTaskIds).catch(() => {})
  }, [activeTaskIds, fetchTaskStatuses])

  useEffect(() => {
    if (open) refresh()
  }, [open, refresh])
  usePolling(refreshActiveStatuses, 5000, open && activeTaskIds.length > 0)

  const activeTasks = tasks.filter((task) => task.status === 'pending' || task.status === 'claimed')
  const completedTasks = tasks.filter((task) => task.status === 'succeeded')
  const failedTasks = tasks.filter((task) => task.status === 'failed' || task.status === 'canceled')

  return (
    <aside className={`tasks-drawer ${open ? 'open' : ''}`} aria-hidden={!open}>
      <header className="tasks-header">
        <h1>{t('tasks.title')}</h1>
        <button type="button" className="topbar-icon" onClick={onClose} aria-label={t('tasks.closeAria')}>
          <X size={28} />
        </button>
      </header>
      {!loading && tasks.length === 0 && <div className="empty-state">{t('tasks.empty')}</div>}
      <section className="task-section">
        <h2>
          <LoaderCircle size={18} />
          {t('tasks.inProgress')}
        </h2>
        <div className="task-list">{activeTasks.map((task) => <TaskCard key={task.id} task={task} />)}</div>
      </section>
      <section className="task-section">
        <h2>
          <CheckCircle2 size={18} />
          {t('tasks.completed')}
        </h2>
        <div className="task-list">{completedTasks.map((task) => <TaskCard key={task.id} task={task} />)}</div>
      </section>
      <section className="task-section">
        <h2>
          <AlertCircle size={18} />
          {t('tasks.failed')}
        </h2>
        <div className="task-list">{failedTasks.map((task) => <TaskCard key={task.id} task={task} />)}</div>
      </section>
    </aside>
  )
}
