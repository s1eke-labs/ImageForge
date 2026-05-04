import { useState } from 'react'
import { useNavigate } from 'react-router'
import { AlertCircle, ChevronDown, Clock3, Hourglass, Image, RefreshCw } from 'lucide-react'
import { useFileObjectURL } from '../hooks/useFileObjectURL.js'
import { useTaskStore } from '../stores/taskStore.js'
import StatusBadge from './StatusBadge.jsx'
import { relativeTime } from '../utils/format.js'

const statusIcon = {
  pending: Hourglass,
  claimed: Image,
  succeeded: Image,
  failed: AlertCircle,
  canceled: AlertCircle,
}

export default function TaskCard({ task }) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [retrying, setRetrying] = useState(false)
  const [retryError, setRetryError] = useState('')
  const retryTask = useTaskStore((state) => state.retryTask)
  const fetchTasks = useTaskStore((state) => state.fetchTasks)
  const done = task.status === 'succeeded'
  const failed = task.status === 'failed' || task.status === 'canceled'
  const thumbPath = task.result_thumb_path || task.result_image_path
  const thumb = useFileObjectURL(thumbPath)
  const Icon = statusIcon[task.status] || Image
  const activity = task.status === 'claimed' ? 'Generating' : task.status === 'pending' ? 'Queued' : relativeTime(task.finished_at || task.created_at)

  const handleClick = () => {
    if (done) navigate(`/artworks/${task.id}`)
    if (failed) setOpen((value) => !value)
  }

  const handleRetry = async (event) => {
    event.stopPropagation()
    if (retrying) return
    setRetrying(true)
    setRetryError('')
    try {
      await retryTask(task.id)
      await fetchTasks()
    } catch (error) {
      setRetryError(error?.message || '重试失败')
      setOpen(true)
    } finally {
      setRetrying(false)
    }
  }

  return (
    <article className={`task-card task-${task.status} ${done || failed ? 'clickable' : ''}`} onClick={handleClick}>
      {thumb ? (
        <img className="task-thumb" src={thumb} alt="" />
      ) : (
        <span className="task-thumb task-thumb-placeholder">
          <Icon size={24} />
        </span>
      )}
      <div className="task-card-body">
        <div className="task-card-head">
          <p>{task.prompt}</p>
          {failed && (
            <button type="button" className="retry-button" onClick={handleRetry} disabled={retrying} aria-label="Retry generation">
              <RefreshCw size={17} className={retrying ? 'spinning' : ''} aria-hidden="true" />
            </button>
          )}
        </div>
        <div className="task-card-meta">
          <StatusBadge status={task.status} />
          <span>
            <Clock3 size={15} />
            {activity}
          </span>
        </div>
        {failed && (
          <button
            type="button"
            className="text-button"
            onClick={(event) => {
              event.stopPropagation()
              setOpen((value) => !value)
            }}
          >
            <ChevronDown size={16} className={open ? 'rotated' : ''} />
            错误详情
          </button>
        )}
        {open && <div className="error-box">{retryError || task.error_message || task.error_code || (task.status === 'canceled' ? '任务已取消' : '任务失败')}</div>}
      </div>
    </article>
  )
}
