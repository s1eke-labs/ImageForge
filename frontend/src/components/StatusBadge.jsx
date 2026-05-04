import { useI18n } from '../i18n/useI18n.js'

const statusMap = {
  pending: { labelKey: 'status.pending', className: 'status-pending' },
  claimed: { labelKey: 'status.claimed', className: 'status-claimed' },
  succeeded: { labelKey: 'status.succeeded', className: 'status-succeeded' },
  failed: { labelKey: 'status.failed', className: 'status-failed' },
  canceled: { labelKey: 'status.canceled', className: 'status-failed' },
}

export default function StatusBadge({ status }) {
  const { t } = useI18n()
  const item = statusMap[status] || { label: status, className: 'status-neutral' }
  return <span className={`status-badge ${item.className}`}>{item.labelKey ? t(item.labelKey) : item.label}</span>
}
