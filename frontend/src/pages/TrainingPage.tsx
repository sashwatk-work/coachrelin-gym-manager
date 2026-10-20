import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import type { PTPackage, PTSession, SessionStatus } from '@/api/types'
import { useDeleteSession, usePackages, useSessions, useSetSessionStatus, useTrainers } from '@/api/hooks'
import { useAuth, useTenant } from '@/lib/auth'
import { can, isTrainerOnly } from '@/lib/rbac'
import { addDays, errorMessage, formatDate, formatMoney, formatTime, titleCase, todayISO } from '@/lib/format'
import {
  ActionMenu,
  Badge,
  Button,
  EmptyState,
  ErrorState,
  Input,
  Loading,
  MenuItem,
  PageHeader,
  Pagination,
  ProgressBar,
  SearchInput,
  Select,
  Table,
  Tabs,
  Td,
  Th,
  statusTone,
} from '@/components/ui'
import { PackageFormModal } from '@/components/forms/PackageForm'
import { SessionFormModal } from '@/components/forms/SessionForm'

type Tab = 'sessions' | 'packages'
const PAGE_SIZE = 25

/** Compact status actions for a session row. */
export function SessionStatusMenu({
  session,
  onSet,
  onEdit,
  onDelete,
}: {
  session: PTSession
  onSet: (s: SessionStatus) => void
  onEdit?: () => void
  onDelete?: () => void
}) {
  return (
    <div className="inline-flex items-center justify-end gap-1">
      {session.status === 'scheduled' && (
        <Button size="sm" variant="secondary" onClick={() => onSet('completed')}>
          Complete
        </Button>
      )}
      <ActionMenu>
        {session.status !== 'completed' && <MenuItem onClick={() => onSet('completed')}>Mark completed</MenuItem>}
        {session.status !== 'no_show' && <MenuItem onClick={() => onSet('no_show')}>Mark no-show</MenuItem>}
        {session.status !== 'cancelled' && <MenuItem onClick={() => onSet('cancelled')}>Cancel session</MenuItem>}
        {session.status !== 'scheduled' && <MenuItem onClick={() => onSet('scheduled')}>Back to scheduled</MenuItem>}
        {onEdit && <MenuItem onClick={onEdit}>Edit…</MenuItem>}
        {onDelete && (
          <MenuItem danger onClick={onDelete}>
            Delete
          </MenuItem>
        )}
      </ActionMenu>
    </div>
  )
}

export default function TrainingPage() {
  const tenant = useTenant()
  const [params, setParams] = useSearchParams()
  const tab = (params.get('tab') as Tab) || 'sessions'
  const setTab = (t: Tab) => setParams({ tab: t }, { replace: true })

  return (
    <div>
      <PageHeader title="Personal training" subtitle="Trainer schedules and the session packages members have bought." />
      <Tabs<Tab>
        value={tab}
        onChange={setTab}
        tabs={[
          { id: 'sessions', label: 'Sessions' },
          { id: 'packages', label: 'Packages' },
        ]}
      />
      <div className="mt-6">{tab === 'sessions' ? <SessionsTab /> : <PackagesTab currency={tenant.currency} />}</div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------
type Range = 'today' | 'week' | 'month' | 'all' | 'custom'

function rangeDates(r: Range, from: string, to: string): { from?: string; to?: string } {
  const today = todayISO()
  switch (r) {
    case 'today':
      return { from: today, to: today }
    case 'week':
      return { from: today, to: addDays(today, 6) }
    case 'month':
      return { from: today, to: addDays(today, 29) }
    case 'custom':
      return { from: from || undefined, to: to || undefined }
    default:
      return {}
  }
}

function SessionsTab() {
  const { user } = useAuth()
  const isTrainer = isTrainerOnly(user?.roles)
  const [range, setRange] = useState<Range>('week')
  const [from, setFrom] = useState(todayISO())
  const [to, setTo] = useState(addDays(todayISO(), 6))
  const [trainerId, setTrainerId] = useState('')
  const [status, setStatus] = useState('')
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [modal, setModal] = useState<{ open: boolean; session?: PTSession }>({ open: false })
  const trainers = useTrainers({ page_size: 200 })
  const setSessionStatus = useSetSessionStatus()
  const del = useDeleteSession()

  const dates = rangeDates(range, from, to)
  const list = useSessions({
    ...dates,
    trainer_id: trainerId || undefined,
    status: status || undefined,
    q: q || undefined,
    order: range === 'all' ? 'desc' : 'asc',
    page,
    page_size: PAGE_SIZE,
  })

  useEffect(() => setPage(1), [range, from, to, trainerId, status, q])

  // Group by day for readability.
  const groups = new Map<string, PTSession[]>()
  for (const s of list.data?.items ?? []) {
    const d = new Date(s.scheduled_at)
    const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    groups.set(key, [...(groups.get(key) ?? []), s])
  }
  const today = todayISO()

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="inline-flex rounded-lg border border-slate-300 bg-white p-0.5 text-sm shadow-sm">
          {(['today', 'week', 'month', 'all', 'custom'] as Range[]).map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => setRange(r)}
              className={`rounded-md px-3 py-1.5 font-medium transition-colors ${range === r ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100'}`}
            >
              {r === 'today' ? 'Today' : r === 'week' ? 'Next 7 days' : r === 'month' ? 'Next 30 days' : r === 'all' ? 'All' : 'Custom'}
            </button>
          ))}
        </div>
        {range === 'custom' && (
          <>
            <Input type="date" value={from} onChange={(e) => setFrom(e.target.value)} className="w-auto" />
            <span className="text-slate-400">to</span>
            <Input type="date" value={to} onChange={(e) => setTo(e.target.value)} className="w-auto" />
          </>
        )}
        {!isTrainer && (
          <Select value={trainerId} onChange={(e) => setTrainerId(e.target.value)} className="w-auto">
            <option value="">All trainers</option>
            {trainers.data?.items.map((t) => (
              <option key={t.id} value={t.id}>
                {t.full_name}
              </option>
            ))}
          </Select>
        )}
        <Select value={status} onChange={(e) => setStatus(e.target.value)} className="w-auto">
          <option value="">Any status</option>
          <option value="scheduled">Scheduled</option>
          <option value="completed">Completed</option>
          <option value="no_show">No-show</option>
          <option value="cancelled">Cancelled</option>
        </Select>
        <SearchInput value={q} onChange={setQ} placeholder="Member or trainer…" className="w-56" />
        <div className="ml-auto">
          <Button onClick={() => setModal({ open: true })}>Schedule session</Button>
        </div>
      </div>

      {list.isLoading ? (
        <Loading />
      ) : list.isError ? (
        <ErrorState message={errorMessage(list.error)} onRetry={() => list.refetch()} />
      ) : groups.size === 0 ? (
        <div className="card">
          <EmptyState title="No sessions in this range" description="Schedule a session from an active PT package." action={<Button onClick={() => setModal({ open: true })}>Schedule session</Button>} />
        </div>
      ) : (
        <div className="space-y-6">
          {[...groups.entries()].map(([day, items]) => (
            <section key={day}>
              <h3 className="mb-2 flex items-baseline gap-2 text-sm font-semibold text-slate-700">
                {formatDate(day, { weekday: 'long', day: 'numeric', month: 'long' })}
                {day === today && <Badge tone="brand">Today</Badge>}
                <span className="text-xs font-normal text-slate-400">
                  {items.length} session{items.length === 1 ? '' : 's'}
                </span>
              </h3>
              <Table>
                <tbody>
                  {items.map((s) => (
                    <tr key={s.id} className={s.status === 'cancelled' ? 'opacity-60' : ''}>
                      <Td className="w-24 whitespace-nowrap font-medium tabular-nums text-slate-900">{formatTime(s.scheduled_at)}</Td>
                      <Td>
                        <Link to={`/members/${s.member_id}`} className="font-medium text-slate-900 hover:bg-signal">
                          {s.member_name}
                        </Link>
                        <div className="text-xs text-slate-500">{s.package_name}</div>
                      </Td>
                      <Td>
                        <Link to={`/trainers/${s.trainer_id}`} className="text-slate-700 hover:bg-signal">
                          {s.trainer_name}
                        </Link>
                      </Td>
                      <Td className="whitespace-nowrap text-slate-500">{s.duration_minutes} min</Td>
                      <Td className="max-w-[16rem] truncate text-slate-500">{s.notes || ''}</Td>
                      <Td>
                        <Badge tone={statusTone(s.status)}>{titleCase(s.status)}</Badge>
                      </Td>
                      <Td className="text-right">
                        <SessionStatusMenu
                          session={s}
                          onSet={(st) => setSessionStatus.mutate({ id: s.id, status: st })}
                          onEdit={() => setModal({ open: true, session: s })}
                          onDelete={
                            isTrainer
                              ? undefined
                              : () => {
                                  if (confirm('Delete this session?')) del.mutate(s.id)
                                }
                          }
                        />
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </section>
          ))}
          <Pagination page={page} pageSize={PAGE_SIZE} total={list.data!.total} onChange={setPage} />
        </div>
      )}

      <SessionFormModal open={modal.open} onClose={() => setModal({ open: false })} session={modal.session} />
    </div>
  )
}

// ---------------------------------------------------------------------------
// Packages
// ---------------------------------------------------------------------------
function PackagesTab({ currency }: { currency: string }) {
  const { user } = useAuth()
  const canSell = can(user?.roles, 'sellPT')
  const isTrainer = isTrainerOnly(user?.roles)
  const [status, setStatus] = useState('active')
  const [trainerId, setTrainerId] = useState('')
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [modal, setModal] = useState<{ open: boolean; pkg?: PTPackage }>({ open: false })
  const [sessionFor, setSessionFor] = useState<PTPackage | null>(null)
  const trainers = useTrainers({ page_size: 200 })
  const list = usePackages({ status: status || undefined, trainer_id: trainerId || undefined, q: q || undefined, page, page_size: PAGE_SIZE })
  useEffect(() => setPage(1), [status, trainerId, q])
  const money = (n: number) => formatMoney(n, currency)

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <Select value={status} onChange={(e) => setStatus(e.target.value)} className="w-auto">
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="completed">Completed</option>
          <option value="expired">Expired</option>
          <option value="cancelled">Cancelled</option>
        </Select>
        {!isTrainer && (
          <Select value={trainerId} onChange={(e) => setTrainerId(e.target.value)} className="w-auto">
            <option value="">All trainers</option>
            {trainers.data?.items.map((t) => (
              <option key={t.id} value={t.id}>
                {t.full_name}
              </option>
            ))}
          </Select>
        )}
        <SearchInput value={q} onChange={setQ} placeholder="Member, trainer or package…" className="w-64" />
        {canSell && (
          <div className="ml-auto">
            <Button onClick={() => setModal({ open: true })}>Sell package</Button>
          </div>
        )}
      </div>

      {list.isLoading ? (
        <Loading />
      ) : list.isError ? (
        <ErrorState message={errorMessage(list.error)} onRetry={() => list.refetch()} />
      ) : list.data!.items.length === 0 ? (
        <div className="card">
          <EmptyState title="No packages sold" description="Sell a PT package from a plan under Plans → Personal training, or customise one for a member." action={canSell ? <Button onClick={() => setModal({ open: true })}>Sell package</Button> : undefined} />
        </div>
      ) : (
        <>
          <Table>
            <thead>
              <tr>
                <Th>Member</Th>
                <Th>Package</Th>
                <Th>Trainer</Th>
                <Th className="w-56">Progress</Th>
                <Th className="text-right">Price</Th>
                <Th className="text-right">Due</Th>
                <Th>Status</Th>
                <Th></Th>
              </tr>
            </thead>
            <tbody>
              {list.data!.items.map((p) => {
                const due = Math.max(0, p.price - p.paid)
                return (
                  <tr key={p.id}>
                    <Td>
                      <Link to={`/members/${p.member_id}`} className="font-medium text-slate-900 hover:bg-signal">
                        {p.member_name}
                      </Link>
                    </Td>
                    <Td>
                      <div>{p.name}</div>
                      {p.pt_plan_name && p.pt_plan_name !== p.name && (
                        <div className="text-xs text-slate-400">from {p.pt_plan_name}</div>
                      )}
                      <div className="text-xs text-slate-400">
                        {formatDate(p.purchased_at)}
                        {p.expires_at ? ` → ${formatDate(p.expires_at)}` : ''}
                      </div>
                    </Td>
                    <Td>
                      <Link to={`/trainers/${p.trainer_id}`} className="text-slate-700 hover:bg-signal">
                        {p.trainer_name}
                      </Link>
                    </Td>
                    <Td>
                      <div className="mb-1 flex justify-between text-xs text-slate-500">
                        <span>
                          {p.sessions_used}/{p.total_sessions} used
                        </span>
                        <span>{p.sessions_remaining} left</span>
                      </div>
                      <ProgressBar value={p.sessions_used} max={p.total_sessions} />
                    </Td>
                    <Td className="text-right tabular-nums">{money(p.price)}</Td>
                    <Td className="text-right tabular-nums">{due > 0 ? <span className="font-medium text-error">{money(due)}</span> : <span className="text-ink">Paid</span>}</Td>
                    <Td>
                      <Badge tone={statusTone(p.status)}>{titleCase(p.status)}</Badge>
                    </Td>
                    <Td className="text-right">
                      <div className="flex justify-end gap-1">
                        {p.status === 'active' && (
                          <Button size="sm" variant="secondary" onClick={() => setSessionFor(p)}>
                            Schedule
                          </Button>
                        )}
                        <Button size="sm" variant="ghost" onClick={() => setModal({ open: true, pkg: p })}>
                          Edit
                        </Button>
                      </div>
                    </Td>
                  </tr>
                )
              })}
            </tbody>
          </Table>
          <Pagination page={page} pageSize={PAGE_SIZE} total={list.data!.total} onChange={setPage} />
        </>
      )}

      <PackageFormModal open={modal.open} onClose={() => setModal({ open: false })} pkg={modal.pkg} />
      <SessionFormModal open={sessionFor !== null} onClose={() => setSessionFor(null)} pkg={sessionFor ?? undefined} />
    </div>
  )
}
