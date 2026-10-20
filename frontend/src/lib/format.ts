import type { ApiError } from '@/api/client'

export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ')
}

const currencyFormatters = new Map<string, Intl.NumberFormat>()

export function formatMoney(amount: number, currency = 'USD'): string {
  let f = currencyFormatters.get(currency)
  if (!f) {
    try {
      f = new Intl.NumberFormat(undefined, { style: 'currency', currency, maximumFractionDigits: 2 })
    } catch {
      f = new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 })
    }
    currencyFormatters.set(currency, f)
  }
  return f.format(amount)
}

/** Formats a YYYY-MM-DD date string without timezone shifting. */
export function formatDate(d: string | null | undefined, opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short', year: 'numeric' }): string {
  if (!d) return '—'
  const [y, m, day] = d.slice(0, 10).split('-').map(Number)
  if (!y || !m || !day) return d
  return new Intl.DateTimeFormat(undefined, opts).format(new Date(y, m - 1, day))
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    hour: 'numeric',
    minute: '2-digit',
  }).format(d)
}

export function formatTime(iso: string): string {
  return new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit' }).format(new Date(iso))
}

/** Today's date as YYYY-MM-DD in the browser's local time. */
export function todayISO(): string {
  return toDateInput(new Date())
}

export function toDateInput(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function addDays(iso: string, n: number): string {
  const [y, m, d] = iso.split('-').map(Number)
  const dt = new Date(y, m - 1, d + n)
  return toDateInput(dt)
}

/** Value for <input type="datetime-local"> from a Date. */
export function toDateTimeLocal(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** Convert a datetime-local string to an RFC3339 timestamp in local tz. */
export function fromDateTimeLocal(v: string): string {
  return new Date(v).toISOString()
}

export function daysUntil(iso: string | null | undefined): number | null {
  if (!iso) return null
  const [y, m, d] = iso.split('-').map(Number)
  const target = new Date(y, m - 1, d)
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  return Math.round((target.getTime() - today.getTime()) / 86_400_000)
}

export function titleCase(s: string): string {
  return s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

export function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? '')
    .join('')
}

export function errorMessage(e: unknown): string {
  if (!e) return ''
  if (typeof e === 'object' && 'message' in e && typeof (e as { message: unknown }).message === 'string') {
    const err = e as ApiError
    if (err.fields) {
      const first = Object.entries(err.fields)[0]
      if (first) return `${titleCase(first[0])} ${first[1]}`
    }
    return err.message
  }
  return String(e)
}
