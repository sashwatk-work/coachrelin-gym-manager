import {
  createContext,
  forwardRef,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from 'react'
import { createPortal } from 'react-dom'
import { cn } from '@/lib/format'

// ---------------------------------------------------------------------------
// Button
// ---------------------------------------------------------------------------
type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  loading?: boolean
}

const variantClass: Record<Variant, string> = {
  primary: 'bg-ink text-paper hover:bg-signal hover:text-signal-ink',
  secondary: 'border border-ink bg-transparent text-ink hover:bg-signal',
  ghost: 'text-ink-soft hover:bg-paper-deep hover:text-ink',
  danger: 'bg-error text-white hover:bg-ink',
}
const sizeClass: Record<Size, string> = {
  sm: 'h-8 px-3 text-[11px]',
  md: 'h-10 px-4 text-[11px]',
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'primary', size = 'md', loading, className, children, disabled, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={cn(
        'inline-flex items-center justify-center gap-2 font-extrabold uppercase tracking-label transition-colors duration-150',
        'focus:outline-none',
        'disabled:cursor-not-allowed disabled:opacity-60',
        variantClass[variant],
        sizeClass[size],
        className,
      )}
      {...rest}
    >
      {loading && <Spinner className="h-3.5 w-3.5" />}
      {children}
    </button>
  )
})

// ---------------------------------------------------------------------------
// Inputs
// ---------------------------------------------------------------------------
export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean
}
export const Input = forwardRef<HTMLInputElement, InputProps>(function Input({ className, invalid, ...rest }, ref) {
  return <input ref={ref} className={cn('input', invalid && 'input-error', className)} {...rest} />
})

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean
}
export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select({ className, invalid, children, ...rest }, ref) {
  return (
    <select ref={ref} className={cn('input pr-8', invalid && 'input-error', className)} {...rest}>
      {children}
    </select>
  )
})

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean
}
export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea({ className, invalid, ...rest }, ref) {
  return <textarea ref={ref} className={cn('input min-h-[80px]', invalid && 'input-error', className)} {...rest} />
})

export function Field({
  label,
  error,
  hint,
  children,
  className,
  htmlFor,
}: {
  label: string
  error?: string
  hint?: string
  children: ReactNode
  className?: string
  htmlFor?: string
}) {
  return (
    <div className={cn('space-y-1.5', className)}>
      <label htmlFor={htmlFor} className="block text-[11px] font-extrabold uppercase tracking-label text-ink-soft">
        {label}
      </label>
      {children}
      {error ? <p className="text-xs text-error">{error}</p> : hint ? <p className="text-xs text-muted">{hint}</p> : null}
    </div>
  )
}

export function Checkbox({ label, ...rest }: InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  const id = useId()
  return (
    <label htmlFor={id} className="inline-flex cursor-pointer items-center gap-2 text-sm text-ink-soft">
      <input id={id} type="checkbox" className="h-4 w-4 rounded-none border-line text-ink accent-signal focus:ring-signal" {...rest} />
      {label}
    </label>
  )
}

// ---------------------------------------------------------------------------
// Badge
// ---------------------------------------------------------------------------
export type Tone = 'green' | 'amber' | 'red' | 'slate' | 'blue' | 'purple' | 'brand'

const toneClass: Record<Tone, string> = {
  green: 'bg-signal/40 text-ink ring-ink/15',
  amber: 'bg-[#f3d27a]/50 text-ink ring-ink/15',
  red: 'bg-[#f3c4bf] text-error ring-error/20',
  slate: 'bg-paper-deep text-ink-soft ring-line',
  blue: 'bg-paper-deep text-ink ring-line',
  purple: 'bg-paper-deep text-ink ring-line',
  brand: 'bg-signal text-signal-ink ring-ink/15',
}

export function Badge({ tone = 'slate', children, className }: { tone?: Tone; children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center px-2 py-0.5 text-[10px] font-extrabold uppercase tracking-wider ring-1 ring-inset',
        toneClass[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}

export function statusTone(status: string): Tone {
  switch (status) {
    case 'active':
    case 'completed':
      return 'green'
    case 'expiring':
    case 'scheduled':
    case 'frozen':
      return 'amber'
    case 'expired':
    case 'cancelled':
    case 'no_show':
      return 'red'
    default:
      return 'slate'
  }
}

// ---------------------------------------------------------------------------
// Misc
// ---------------------------------------------------------------------------
export function Spinner({ className }: { className?: string }) {
  return (
    <svg className={cn('animate-spin', className ?? 'h-5 w-5')} viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
  )
}

export function Loading({ label = 'Loading…' }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-16 text-sm text-muted">
      <Spinner className="h-4 w-4" /> {label}
    </div>
  )
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="card flex flex-col items-center gap-3 px-6 py-12 text-center">
      <p className="text-sm text-error">{message}</p>
      {onRetry && (
        <Button variant="secondary" size="sm" onClick={onRetry}>
          Retry
        </Button>
      )}
    </div>
  )
}

export function EmptyState({
  title,
  description,
  action,
  icon,
}: {
  title: string
  description?: string
  action?: ReactNode
  icon?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center px-6 py-14 text-center">
      {icon && <div className="mb-3 text-muted">{icon}</div>}
      <p className="text-sm font-semibold text-ink">{title}</p>
      {description && <p className="mt-1 max-w-sm text-sm text-muted">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
}) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 className="font-display text-4xl font-semibold uppercase leading-none tracking-tight text-ink">{title}</h1>
        {subtitle && <p className="mt-2 text-sm text-muted">{subtitle}</p>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}

export function StatCard({
  label,
  value,
  sub,
  tone,
}: {
  label: string
  value: ReactNode
  sub?: ReactNode
  tone?: 'default' | 'warn' | 'danger' | 'good'
}) {
  const accent =
    tone === 'warn' ? 'text-ink' : tone === 'danger' ? 'text-error' : tone === 'good' ? 'text-ink' : 'text-ink'
  return (
    <div className="card px-5 py-4">
      <p className="flex items-center gap-2 text-[10px] font-extrabold uppercase tracking-label text-muted">
        <span className="inline-block h-[2px] w-4 bg-signal" aria-hidden />
        {label}
      </p>
      <p className={cn('mt-2 font-display text-3xl font-semibold tabular-nums leading-none tracking-tight', accent)}>{value}</p>
      {sub && <p className="mt-2 text-xs text-muted">{sub}</p>}
    </div>
  )
}

export function Avatar({ name, size = 'md' }: { name: string; size?: 'sm' | 'md' | 'lg' }) {
  const init = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase())
    .join('')
  const s = size === 'sm' ? 'h-7 w-7 text-[10px]' : size === 'lg' ? 'h-14 w-14 text-lg' : 'h-9 w-9 text-xs'
  return (
    <div className={cn('flex shrink-0 items-center justify-center rounded-full bg-ink font-display font-bold text-signal', s)}>
      {init}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Table
// ---------------------------------------------------------------------------
export function Table({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('card overflow-hidden', className)}>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">{children}</table>
      </div>
    </div>
  )
}

export function Th({ children, className }: { children?: ReactNode; className?: string }) {
  return (
    <th className={cn('whitespace-nowrap bg-paper px-4 py-2.5 text-[10px] font-extrabold uppercase tracking-label text-muted', className)}>
      {children}
    </th>
  )
}

export function Td({ children, className, colSpan }: { children?: ReactNode; className?: string; colSpan?: number }) {
  return (
    <td colSpan={colSpan} className={cn('border-t border-line px-4 py-3 align-middle text-ink-soft', className)}>
      {children}
    </td>
  )
}

export function Pagination({
  page,
  pageSize,
  total,
  onChange,
}: {
  page: number
  pageSize: number
  total: number
  onChange: (page: number) => void
}) {
  const pages = Math.max(1, Math.ceil(total / pageSize))
  if (pages <= 1) return null
  const from = (page - 1) * pageSize + 1
  const to = Math.min(total, page * pageSize)
  return (
    <div className="flex items-center justify-between px-1 pt-3 text-xs text-muted">
      <span>
        Showing {from}–{to} of {total}
      </span>
      <div className="flex items-center gap-1">
        <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => onChange(page - 1)}>
          Previous
        </Button>
        <span className="px-2">
          Page {page} of {pages}
        </span>
        <Button variant="secondary" size="sm" disabled={page >= pages} onClick={() => onChange(page + 1)}>
          Next
        </Button>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Tabs
// ---------------------------------------------------------------------------
export function Tabs<T extends string>({
  tabs,
  value,
  onChange,
}: {
  tabs: Array<{ id: T; label: string; count?: number }>
  value: T
  onChange: (id: T) => void
}) {
  return (
    <div className="border-b border-line">
      <nav className="-mb-px flex gap-6" aria-label="Tabs">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => onChange(t.id)}
            className={cn(
              'flex items-center gap-2 whitespace-nowrap border-b-2 px-1 py-3 text-[11px] font-extrabold uppercase tracking-label transition-colors',
              value === t.id
                ? 'border-signal text-ink'
                : 'border-transparent text-muted hover:border-line hover:text-ink',
            )}
          >
            {t.label}
            {t.count !== undefined && (
              <span
                className={cn(
                  'px-2 py-0.5 text-[10px] tabular-nums',
                  value === t.id ? 'bg-signal text-ink' : 'bg-paper-deep text-ink-soft',
                )}
              >
                {t.count}
              </span>
            )}
          </button>
        ))}
      </nav>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Action menu (portaled so table overflow cannot clip it)
// ---------------------------------------------------------------------------
const ActionMenuCtx = createContext<{ close: () => void }>({ close: () => {} })

export function ActionMenu({
  label = 'More actions',
  children,
}: {
  label?: string
  children: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const btnRef = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState({ top: 0, left: 0 })

  const place = () => {
    const r = btnRef.current?.getBoundingClientRect()
    if (!r) return
    const width = menuRef.current?.offsetWidth || 188
    const height = menuRef.current?.offsetHeight || 220
    const left = Math.min(Math.max(8, r.right - width), window.innerWidth - width - 8)
    const top = r.bottom + 4 + height > window.innerHeight - 8 ? r.top - height - 4 : r.bottom + 4
    setPos({ top: Math.max(8, top), left })
  }

  useEffect(() => {
    if (!open) return
    const id = requestAnimationFrame(() => {
      place()
      requestAnimationFrame(place)
    })
    const onDoc = (e: MouseEvent) => {
      const t = e.target as Node
      if (btnRef.current?.contains(t) || menuRef.current?.contains(t)) return
      setOpen(false)
    }
    const onReposition = () => place()
    document.addEventListener('mousedown', onDoc)
    window.addEventListener('scroll', onReposition, true)
    window.addEventListener('resize', onReposition)
    return () => {
      cancelAnimationFrame(id)
      document.removeEventListener('mousedown', onDoc)
      window.removeEventListener('scroll', onReposition, true)
      window.removeEventListener('resize', onReposition)
    }
  }, [open])

  return (
    <>
      <button
        ref={btnRef}
        type="button"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="p-1.5 text-muted hover:bg-signal hover:text-ink"
      >
        <svg className="h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden>
          <path d="M10 6a1.5 1.5 0 110-3 1.5 1.5 0 010 3zm0 5.5a1.5 1.5 0 110-3 1.5 1.5 0 010 3zm0 5.5a1.5 1.5 0 110-3 1.5 1.5 0 010 3z" />
        </svg>
      </button>
      {open &&
        createPortal(
          <ActionMenuCtx.Provider value={{ close: () => setOpen(false) }}>
            <div
              ref={menuRef}
              role="menu"
              style={{ position: 'fixed', top: pos.top, left: pos.left, width: 188, zIndex: 70 }}
              className="border border-line bg-raised py-1 shadow-xl"
            >
              {children}
            </div>
          </ActionMenuCtx.Provider>,
          document.body,
        )}
    </>
  )
}

export function MenuItem({
  children,
  onClick,
  danger,
}: {
  children: ReactNode
  onClick: () => void
  danger?: boolean
}) {
  const { close } = useContext(ActionMenuCtx)
  return (
    <button
      type="button"
      role="menuitem"
      className={cn('block w-full px-3 py-2 text-left text-sm hover:bg-signal', danger && 'text-error')}
      onClick={() => {
        close()
        onClick()
      }}
    >
      {children}
    </button>
  )
}

// ---------------------------------------------------------------------------
// Modal
// ---------------------------------------------------------------------------
export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = 'md',
}: {
  open: boolean
  onClose: () => void
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
  size?: 'sm' | 'md' | 'lg'
}) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [open, onClose])

  if (!open) return null
  const width = size === 'sm' ? 'max-w-sm' : size === 'lg' ? 'max-w-2xl' : 'max-w-lg'
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end justify-center sm:items-center" role="dialog" aria-modal="true" aria-label={title}>
      <div className="absolute inset-0 bg-ink/50" onClick={onClose} />
      <div className={cn('relative m-0 flex max-h-[92vh] w-full flex-col border border-line bg-raised shadow-xl sm:m-4', width)}>
        <div className="flex items-start justify-between gap-4 border-b border-line px-6 py-4">
          <div>
            <h2 className="font-display text-2xl font-semibold uppercase tracking-tight text-ink">{title}</h2>
            {description && <p className="mt-0.5 text-sm text-muted">{description}</p>}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1 text-muted hover:bg-signal hover:text-ink"
            aria-label="Close"
          >
            <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
              <path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
            </svg>
          </button>
        </div>
        <div className="overflow-y-auto px-6 py-5">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-line px-6 py-4">{footer}</div>}
      </div>
    </div>,
    document.body,
  )
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel = 'Delete',
  loading,
  danger = true,
}: {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  description?: string
  confirmLabel?: string
  loading?: boolean
  danger?: boolean
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button variant={danger ? 'danger' : 'primary'} onClick={onConfirm} loading={loading}>
            {confirmLabel}
          </Button>
        </>
      }
    >
      <p className="text-sm text-ink-soft">{description}</p>
    </Modal>
  )
}

export function FormError({ message }: { message?: string | null }) {
  if (!message) return null
  return <div className="border border-error/30 bg-[#f3c4bf] px-3 py-2 text-sm text-error">{message}</div>
}

export function SearchInput({
  value,
  onChange,
  placeholder = 'Search…',
  className,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  className?: string
}) {
  return (
    <div className={cn('relative', className)}>
      <svg className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" viewBox="0 0 20 20" fill="currentColor">
        <path
          fillRule="evenodd"
          d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z"
          clipRule="evenodd"
        />
      </svg>
      <input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="input pl-9"
      />
    </div>
  )
}

export function ProgressBar({ value, max, className }: { value: number; max: number; className?: string }) {
  const pct = max > 0 ? Math.min(100, Math.round((value / max) * 100)) : 0
  return (
    <div className={cn('h-1.5 w-full overflow-hidden bg-paper-deep', className)}>
      <div className={cn('h-full', pct >= 100 ? 'bg-ink' : 'bg-signal')} style={{ width: `${pct}%` }} />
    </div>
  )
}
