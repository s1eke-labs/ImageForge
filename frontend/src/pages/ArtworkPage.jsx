import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ArrowLeft, Clock3, Download, Ruler } from 'lucide-react'
import { useFileObjectURL } from '../hooks/useFileObjectURL.js'
import { useTaskStore } from '../stores/taskStore.js'

function resolutionLabel(task) {
  if (task.result_width && task.result_height) {
    return `${task.result_width} x ${task.result_height}`
  }
  return task.size && task.size.includes('x') ? task.size : '-'
}

function durationLabel(task) {
  const created = task.created_at ? new Date(task.created_at).getTime() : 0
  const finished = task.finished_at ? new Date(task.finished_at).getTime() : 0
  const seconds = created && finished && finished >= created ? (finished - created) / 1000 : task.duration_seconds
  if (!seconds) return '-'
  if (seconds < 60) return `${Math.max(1, Math.round(seconds))} 秒`
  const minutes = Math.floor(seconds / 60)
  const rest = Math.round(seconds % 60)
  if (minutes < 60) return rest ? `${minutes} 分 ${rest} 秒` : `${minutes} 分`
  const hours = Math.floor(minutes / 60)
  const minuteRest = minutes % 60
  return minuteRest ? `${hours} 小时 ${minuteRest} 分` : `${hours} 小时`
}

export default function ArtworkPage() {
  const { id } = useParams()
  const getTask = useTaskStore((state) => state.getTask)
  const [task, setTask] = useState(null)
  const imagePath = task?.result_image_path || ''
  const image = useFileObjectURL(imagePath)

  useEffect(() => {
    getTask(id).then(setTask).catch(() => setTask(null))
  }, [getTask, id])

  if (!task) {
    return <main className="artwork-page page">加载中</main>
  }

  return (
    <main className="artwork-page page">
      <header className="detail-header">
        <Link to="/create" className="topbar-icon" aria-label="Back to generator">
          <ArrowLeft size={30} />
        </Link>
        <h1>Image Result</h1>
        {image && (
          <a className="topbar-icon detail-download" href={image} download="imageforge-result.png" aria-label="Download original image">
            <Download size={24} />
          </a>
        )}
      </header>
      <div className="artwork-stage">
        <div className="artwork-frame" aria-label="Generated image">
          {image && (
            <img src={image} alt="" />
          )}
        </div>
      </div>
      <section className="result-meta-card">
        <div className="result-prompt">
          <span>提示词</span>
          <p>{task.prompt}</p>
        </div>
        <div className="result-facts">
          <div>
            <span>分辨率</span>
            <p>
              <Ruler size={17} />
              {resolutionLabel(task)}
            </p>
          </div>
          <div>
            <span>生成时间</span>
            <p>
              <Clock3 size={17} />
              {durationLabel(task)}
            </p>
          </div>
        </div>
      </section>
    </main>
  )
}
