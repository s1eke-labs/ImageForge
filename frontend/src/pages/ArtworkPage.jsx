import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { ArrowLeft, Clock3, Download, Ruler } from 'lucide-react'
import { useFileObjectURL } from '../hooks/useFileObjectURL.js'
import { useI18n } from '../i18n/useI18n.js'
import { useTaskStore } from '../stores/taskStore.js'
import { duration } from '../utils/format.js'

function resolutionLabel(task) {
  if (task.result_width && task.result_height) {
    return `${task.result_width} x ${task.result_height}`
  }
  return task.size && task.size.includes('x') ? task.size : '-'
}

function durationLabel(task, locale) {
  const created = task.created_at ? new Date(task.created_at).getTime() : 0
  const finished = task.finished_at ? new Date(task.finished_at).getTime() : 0
  const seconds = created && finished && finished >= created ? (finished - created) / 1000 : task.duration_seconds
  return duration(seconds, locale)
}

export default function ArtworkPage() {
  const { id } = useParams()
  const getTask = useTaskStore((state) => state.getTask)
  const [task, setTask] = useState(null)
  const imagePath = task?.result_image_path || ''
  const image = useFileObjectURL(imagePath)
  const { locale, t } = useI18n()

  useEffect(() => {
    getTask(id).then(setTask).catch(() => setTask(null))
  }, [getTask, id])

  if (!task) {
    return <main className="artwork-page page">{t('artwork.loading')}</main>
  }

  return (
    <main className="artwork-page page">
      <header className="detail-header">
        <Link to="/create" className="topbar-icon" aria-label={t('artwork.backAria')}>
          <ArrowLeft size={30} />
        </Link>
        <h1>{t('artwork.title')}</h1>
        {image && (
          <a className="topbar-icon detail-download" href={image} download="imageforge-result.png" aria-label={t('artwork.downloadAria')}>
            <Download size={24} />
          </a>
        )}
      </header>
      <div className="artwork-stage">
        <div className="artwork-frame" aria-label={t('artwork.imageAria')}>
          {image && (
            <img src={image} alt="" />
          )}
        </div>
      </div>
      <section className="result-meta-card">
        <div className="result-prompt">
          <span>{t('artwork.prompt')}</span>
          <p>{task.prompt}</p>
        </div>
        <div className="result-facts">
          <div>
            <span>{t('artwork.resolution')}</span>
            <p>
              <Ruler size={17} />
              {resolutionLabel(task)}
            </p>
          </div>
          <div>
            <span>{t('artwork.duration')}</span>
            <p>
              <Clock3 size={17} />
              {durationLabel(task, locale)}
            </p>
          </div>
        </div>
      </section>
    </main>
  )
}
