import { useEffect, useRef, useState } from 'react'
import { ChevronDown, Menu, Palette, Plus, Sparkles, X } from 'lucide-react'
import LanguageSwitcher from '../components/LanguageSwitcher.jsx'
import TaskListPage from './TaskListPage.jsx'
import logo from '../assets/logo.svg'
import { useI18n } from '../i18n/useI18n.js'
import { useTaskStore } from '../stores/taskStore.js'

const sizes = [
  { value: 'auto', labelKey: 'create.size.auto', icon: 'auto' },
  { value: '1:1', label: '1:1', sizes: { low: '1024x1024', medium: '1536x1536', high: '2048x2048' } },
  { value: '3:4', label: '3:4', ratio: [3, 4], sizes: { low: '768x1024', medium: '1536x2048', high: '2304x3072' } },
  { value: '4:3', label: '4:3', ratio: [4, 3], sizes: { low: '1024x768', medium: '2048x1536', high: '3072x2304' } },
  { value: '9:16', label: '9:16', ratio: [9, 16], sizes: { low: '720x1280', medium: '1152x2048', high: '1440x2560' } },
  { value: '16:9', label: '16:9', ratio: [16, 9], sizes: { low: '1280x720', medium: '2048x1152', high: '2560x1440' } },
  { value: '21:9', label: '21:9', ratio: [21, 9], sizes: { low: '1344x576', medium: '2016x864', high: '3360x1440' } },
]
const qualities = [
  { value: 'high', labelKey: 'create.quality.high', level: 3 },
  { value: 'medium', labelKey: 'create.quality.medium', level: 2 },
  { value: 'low', labelKey: 'create.quality.low', level: 1 },
]
const minPixels = 655360
const maxPixels = 8294400

function selectedOption(options, value) {
  return options.find((item) => item.value === value || item.id === value) || options[0]
}

function optionLabel(option, t) {
  return option.labelKey ? t(option.labelKey) : option.label
}

function parseSize(value) {
  const [width, height] = value.split('x').map((part) => Number.parseInt(part, 10))
  return { width, height }
}

function validateImageSize(width, height, t) {
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    throw new Error(t('error.invalidResolution'))
  }
  if (width % 16 !== 0 || height % 16 !== 0) {
    throw new Error(t('error.sizeMultiple'))
  }
  if (Math.max(width, height) > 3840) {
    throw new Error(t('error.maxSide'))
  }
  if (Math.max(width, height) / Math.min(width, height) > 3) {
    throw new Error(t('error.maxRatio'))
  }
  const pixels = width * height
  if (pixels < minPixels || pixels > maxPixels) {
    throw new Error(t('error.pixelRange'))
  }
}

function sizeForSelection(size, quality, referenceDimensions, t) {
  const option = selectedOption(sizes, size)
  if (option.value === 'auto') {
    return 'auto'
  }
  const resolved = option.sizes?.[quality] || option.value
  const { width, height } = parseSize(resolved)
  validateImageSize(width, height, t)
  return resolved
}

function readImageDimensions(file, t) {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const image = new window.Image()
    image.onload = () => {
      URL.revokeObjectURL(url)
      resolve({ width: image.naturalWidth, height: image.naturalHeight })
    }
    image.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error(t('error.readReferenceDimensions')))
    }
    image.src = url
  })
}

function imageFileFromClipboard(event) {
  const items = Array.from(event.clipboardData?.items || [])
  const imageItem = items.find((item) => item.kind === 'file' && item.type.startsWith('image/'))
  const file = imageItem?.getAsFile()
  if (!file) return null
  const extension = file.type.split('/')[1] || 'png'
  return new window.File([file], file.name || `pasted-reference.${extension}`, { type: file.type })
}

function normalizeStyles(data, t) {
  const items = Array.isArray(data) ? data : data?.styles
  if (!Array.isArray(items)) return []
  return items
    .map((item, index) => ({
      id: String(item.id || item.label || `style-${index}`),
      label: String(item.label || item.name || item.id || `${t('create.style')} ${index + 1}`),
      prompt: String(item.prompt || ''),
      preview: item.preview ? String(item.preview) : '',
    }))
    .filter((item) => item.label.trim())
}

function RatioGlyph({ option }) {
  if (option.icon === 'auto') {
    return (
      <span className="ratio-glyph" aria-hidden="true">
        <span className="ratio-auto-shape" />
      </span>
    )
  }
  const [width, height] = option.ratio || option.label.split(':').map(Number)
  return (
    <span className="ratio-glyph" aria-hidden="true">
      <span className="ratio-shape" style={{ '--ratio-w': width || 1, '--ratio-h': height || 1 }} />
    </span>
  )
}

function QualityGlyph({ option }) {
  const level = option.level || 1
  return (
    <span className="quality-glyph" aria-hidden="true">
      <i className={level >= 1 ? 'active' : ''} />
      <i className={level >= 2 ? 'active' : ''} />
      <i className={level >= 3 ? 'active' : ''} />
    </span>
  )
}

function StyleGlyph() {
  return (
    <span className="style-glyph" aria-hidden="true">
      <Palette size={22} />
    </span>
  )
}

function StylePreview({ option }) {
  if (option.preview) {
    return (
      <span className="style-preview" aria-hidden="true">
        <img src={option.preview} alt="" />
      </span>
    )
  }
  return <StyleGlyph />
}

export default function CreatePage() {
  const { t } = useI18n()
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState('1:1')
  const [quality, setQuality] = useState('low')
  const [file, setFile] = useState(null)
  const [preview, setPreview] = useState('')
  const [referenceDimensions, setReferenceDimensions] = useState(null)
  const [styles, setStyles] = useState([])
  const [selectedStyleId, setSelectedStyleId] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [tasksOpen, setTasksOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState('')
  const fileInputRef = useRef(null)
  const fileVersionRef = useRef(0)
  const promptToolsRef = useRef(null)
  const createTask = useTaskStore((state) => state.createTask)
  const fetchTasks = useTaskStore((state) => state.fetchTasks)
  const currentSize = selectedOption(sizes, size)
  const currentQuality = selectedOption(qualities, quality)

  useEffect(() => {
    if (!file) {
      setPreview('')
      return undefined
    }
    const url = URL.createObjectURL(file)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [file])

  useEffect(() => {
    let active = true
    window.fetch('/prompt-styles/styles.json', { cache: 'no-store' })
      .then((response) => (response.ok ? response.json() : []))
      .then((data) => {
        if (active) setStyles(normalizeStyles(data, t))
      })
      .catch(() => {
        if (active) setStyles([])
      })
    return () => {
      active = false
    }
  }, [t])

  useEffect(() => {
    if (!settingsOpen) return undefined
    const closeOnOutsidePress = (event) => {
      if (!promptToolsRef.current?.contains(event.target)) {
        setSettingsOpen('')
      }
    }
    document.addEventListener('pointerdown', closeOnOutsidePress)
    return () => document.removeEventListener('pointerdown', closeOnOutsidePress)
  }, [settingsOpen])

  const submit = async (event) => {
    event.preventDefault()
    setLoading(true)
    setError('')
    try {
      const finalDimensions = file && !referenceDimensions ? await readImageDimensions(file, t) : referenceDimensions
      if (file && !referenceDimensions) setReferenceDimensions(finalDimensions)
      const form = new FormData()
      form.set('prompt', prompt.trim())
      form.set('size', sizeForSelection(size, quality, finalDimensions, t))
      if (file) form.set('reference_image', file)
      await createTask(form)
      fetchTasks().catch(() => {})
      setTasksOpen(true)
    } catch (err) {
      setError(err?.message || t('error.createTask'))
    } finally {
      setLoading(false)
    }
  }

  const setReferenceFile = (selected) => {
    const version = fileVersionRef.current + 1
    fileVersionRef.current = version
    setFile(selected)
    setReferenceDimensions(null)
    setError('')
    if (selected) {
      readImageDimensions(selected, t)
        .then((dimensions) => {
          if (fileVersionRef.current === version) setReferenceDimensions(dimensions)
        })
        .catch((err) => {
          if (fileVersionRef.current === version) setError(err?.message || t('error.readReferenceDimensions'))
        })
    }
  }

  const selectReferenceImage = (event) => {
    setReferenceFile(event.target.files?.[0] || null)
    event.target.value = ''
  }

  const pasteReferenceImage = (event) => {
    const pasted = imageFileFromClipboard(event)
    if (!pasted) return
    event.preventDefault()
    setReferenceFile(pasted)
  }

  const clearReferenceImage = () => {
    fileVersionRef.current += 1
    setFile(null)
    setReferenceDimensions(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  return (
    <div className="page create-page" onPaste={pasteReferenceImage}>
      <header className="app-topbar">
        <div className="brand-mark">
          <img src={logo} alt="" />
          <span>ImageForge</span>
        </div>
        <div className="topbar-actions">
          <LanguageSwitcher />
          <button type="button" className="topbar-icon" onClick={() => setTasksOpen(true)} aria-label={t('create.tasksAria')}>
            <Menu size={28} />
          </button>
        </div>
      </header>
      <form className="create-form" onSubmit={submit}>
        <div className="field">
          <span className="prompt-label-row">
            <label htmlFor="prompt-input">{t('create.promptLabel')}</label>
            <span className="reference-control">
              <span className={`reference-picker ${preview ? 'has-preview' : ''}`} aria-label={t('create.referenceUploadAria')}>
                {preview ? <img src={preview} alt="" /> : <Plus size={28} />}
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  onChange={selectReferenceImage}
                />
              </span>
              {preview && (
                <button type="button" className="reference-clear" onClick={clearReferenceImage} aria-label={t('create.referenceRemoveAria')}>
                  <X size={16} />
                </button>
              )}
            </span>
          </span>
          <span className="prompt-field">
            <textarea
              id="prompt-input"
              value={prompt}
              onChange={(event) => {
                setPrompt(event.target.value)
                setSelectedStyleId('')
              }}
              placeholder={t('create.promptPlaceholder')}
              required
              rows={10}
            />
            <span className="prompt-bottom-tools">
              <span className="prompt-selects" ref={promptToolsRef}>
                <span className={`prompt-select style-select ${settingsOpen === 'style' ? 'open' : ''}`}>
                  {settingsOpen === 'style' && (
                    <span className="prompt-menu style-menu">
                      {styles.length === 0 && (
                        <span className="prompt-menu-empty">{t('create.noStyles')}</span>
                      )}
                      {styles.map((item) => (
                        <button
                          key={item.id}
                          type="button"
                          className={selectedStyleId === item.id ? 'selected' : ''}
                          onClick={() => {
                            if (selectedStyleId === item.id) {
                              setSelectedStyleId('')
                              setPrompt('')
                            } else {
                              setSelectedStyleId(item.id)
                              setPrompt(item.prompt)
                            }
                            setSettingsOpen('')
                          }}
                        >
                          <StylePreview option={item} />
                          <span>{item.label}</span>
                        </button>
                      ))}
                    </span>
                  )}
                  <button
                    type="button"
                    className="prompt-select-button"
                    onClick={() => setSettingsOpen((value) => (value === 'style' ? '' : 'style'))}
                    aria-haspopup="menu"
                    aria-expanded={settingsOpen === 'style'}
                  >
                    <span>{t('create.style')}</span>
                    <ChevronDown size={19} aria-hidden="true" />
                  </button>
                </span>
                <span className={`prompt-select ratio-select ${settingsOpen === 'size' ? 'open' : ''}`}>
                  {settingsOpen === 'size' && (
                    <span className="prompt-menu ratio-menu">
                      {sizes.map((item) => (
                        <button
                          key={item.value}
                          type="button"
                          className={size === item.value ? 'selected' : ''}
                          onClick={() => {
                            setSize(item.value)
                            setSettingsOpen('')
                          }}
                        >
                          <RatioGlyph option={item} />
                          <span>{optionLabel(item, t)}</span>
                        </button>
                      ))}
                    </span>
                  )}
                  <button
                    type="button"
                    className="prompt-select-button"
                    onClick={() => setSettingsOpen((value) => (value === 'size' ? '' : 'size'))}
                    aria-haspopup="menu"
                    aria-expanded={settingsOpen === 'size'}
                  >
                    <RatioGlyph option={currentSize} />
                    <span>{optionLabel(currentSize, t)}</span>
                    <ChevronDown size={19} aria-hidden="true" />
                  </button>
                </span>
                <span className={`prompt-select quality-select ${settingsOpen === 'quality' ? 'open' : ''}`}>
                  {settingsOpen === 'quality' && (
                    <span className="prompt-menu quality-menu">
                      {qualities.map((item) => (
                        <button
                          key={item.value}
                          type="button"
                          className={quality === item.value ? 'selected' : ''}
                          onClick={() => {
                            setQuality(item.value)
                            setSettingsOpen('')
                          }}
                        >
                          <QualityGlyph option={item} />
                          <span>{optionLabel(item, t)}</span>
                        </button>
                      ))}
                    </span>
                  )}
                  <button
                    type="button"
                    className="prompt-select-button"
                    onClick={() => setSettingsOpen((value) => (value === 'quality' ? '' : 'quality'))}
                    aria-haspopup="menu"
                    aria-expanded={settingsOpen === 'quality'}
                  >
                    <span>{optionLabel(currentQuality, t)}</span>
                    <ChevronDown size={19} aria-hidden="true" />
                  </button>
                </span>
              </span>
            </span>
          </span>
        </div>
        {error && <div className="form-error">{error}</div>}
        <button className="primary-button sticky-action" type="submit" disabled={loading || !prompt.trim()}>
          <Sparkles size={18} />
          {loading ? t('create.generating') : t('create.generate')}
        </button>
      </form>
      <div className={`tasks-scrim ${tasksOpen ? 'open' : ''}`} onClick={() => setTasksOpen(false)} />
      <TaskListPage open={tasksOpen} onClose={() => setTasksOpen(false)} />
    </div>
  )
}
