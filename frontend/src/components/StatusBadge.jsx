const statusMap = {
  pending: { label: 'Queued', className: 'status-pending' },
  claimed: { label: 'Generating', className: 'status-claimed' },
  succeeded: { label: 'Completed', className: 'status-succeeded' },
  failed: { label: 'Failed', className: 'status-failed' },
  canceled: { label: 'Canceled', className: 'status-failed' },
}

export default function StatusBadge({ status }) {
  const item = statusMap[status] || { label: status, className: 'status-neutral' }
  return <span className={`status-badge ${item.className}`}>{item.label}</span>
}
