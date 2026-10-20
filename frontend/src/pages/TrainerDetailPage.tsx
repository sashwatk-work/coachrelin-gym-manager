import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import type { PTSession } from '@/api/types'
import { useDeleteTrainer, useSetSessionStatus, useTrainer, useUsers } from '@/api/hooks'
import { useAuth, useTenant } from '@/lib/auth'
import { can } from '@/lib/rbac'
import { errorMessage, formatDate, formatDateTime, formatMoney, titleCase } from '@/lib/format'
import { Avatar, Badge, Button, ConfirmDialog, EmptyState, ErrorState, Loading, ProgressBar, StatCard, Table, Tabs, Td, Th, statusTone } from '@/components/ui'
import { TrainerFormModal } from '@/components/forms/TrainerForm'
import { UserFormModal, loginForTrainer } from '@/components/forms/UserForm'
import { SessionFormModal } from '@/components/forms/SessionForm'
import { SessionStatusMenu } from './TrainingPage'

type Tab = 'upcoming' | 'clients' | 'history'

export default function TrainerDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const tenant = useTenant()
  const { user } = useAuth()
  const canManage = can(user?.roles, 'manageTrainers')
  const canCreateLogin = can(user?.roles, 'manageUsers')
  const q = useTrainer(id)
  const users = useUsers({ enabled: canCreateLogin })
  const del = useDeleteTrainer()
  const setStatus = useSetSessionStatus()
  const [tab, setTab] = useState<Tab>('upcoming')
  const [editing, setEditing] = useState(false)
  const [loginOpen, setLoginOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')
  const [editSession, setEditSession] = useState<PTSession | null>(null)

  if (q.isLoading) return <Loading />
  if (q.isError || !q.data) return <ErrorState message={errorMessage(q.error)} onRetry={() => q.refetch()} />
  const t = q.data

  const now = Date.now()
  const upcoming = t.sessions.filter((s) => s.status === 'scheduled' && new Date(s.scheduled_at).getTime() >= now - 3600_000).sort((a, b) => a.scheduled_at.localeCompare(b.scheduled_at))
  const history = t.sessions.filter((s) => !upcoming.includes(s))
  const activePkgs = t.packages.filter((p) => p.status === 'active')
  const completedThisMonth = t.sessions.filter((s) => s.status === 'completed' && new Date(s.scheduled_at).getMonth() === new Date().getMonth() && new Date(s.scheduled_at).getFullYear() === new Date().getFullYear()).length
  const login = loginForTrainer(users.data?.items, t.id)

  const sessionRows = (rows: PTSession[]) => (
    <Table>
      <thead>
        <tr>
          <Th>When</Th>
          <Th>Member</Th>
          <Th>Package</Th>
          <Th>Duration</Th>
          <Th>Status</Th>
          <Th></Th>
        </tr>
      </thead>
      <tbody>
        {rows.map((s) => (
          <tr key={s.id}>
            <Td className="whitespace-nowrap font-medium text-slate-900">{formatDateTime(s.scheduled_at)}</Td>
            <Td>
              <Link to={`/members/${s.member_id}`} className="text-slate-800 hover:bg-signal">
                {s.member_name}
              </Link>
            </Td>
            <Td className="text-slate-500">{s.package_name}</Td>
            <Td className="text-slate-500">{s.duration_minutes} min</Td>
            <Td>
              <Badge tone={statusTone(s.status)}>{titleCase(s.status)}</Badge>
            </Td>
            <Td className="text-right">
              <SessionStatusMenu session={s} onSet={(st) => setStatus.mutate({ id: s.id, status: st })} onEdit={() => setEditSession(s)} />
            </Td>
          </tr>
        ))}
      </tbody>
    </Table>
  )

  return (
    <div>
      <nav className="mb-4 text-sm text-slate-500">
        <Link to="/trainers" className="hover:text-slate-800">
          Trainers
        </Link>
        <span className="mx-2">/</span>
        <span className="text-slate-800">{t.full_name}</span>
      </nav>

      <div className="card mb-6 flex flex-wrap items-center gap-5 px-6 py-5">
        <Avatar name={t.full_name} size="lg" />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-xl font-semibold tracking-tight">{t.full_name}</h1>
            <Badge tone={t.active ? 'green' : 'slate'}>{t.active ? 'Active' : 'Inactive'}</Badge>
          </div>
          <p className="mt-1 text-sm text-slate-500">{[t.phone, t.email].filter(Boolean).join(' · ') || 'No contact details'}</p>
          {canCreateLogin && login && <p className="mt-1 text-sm text-slate-500">Signs in as {login.email}</p>}
          {t.specialties.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1">
              {t.specialties.map((s) => (
                <span key={s} className="rounded-md bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                  {s}
                </span>
              ))}
            </div>
          )}
        </div>
        {(canManage || canCreateLogin) && (
          <div className="flex gap-2">
            {canCreateLogin && !login && (
              <Button variant="secondary" onClick={() => setLoginOpen(true)}>
                Create login
              </Button>
            )}
            {canManage && (
              <>
                <Button variant="secondary" onClick={() => setEditing(true)}>
                  Edit
                </Button>
                <Button
                  variant="ghost"
                  className="text-error hover:bg-[#f3c4bf]"
                  onClick={() => {
                    setDeleteError('')
                    setDeleting(true)
                  }}
                >
                  Delete
                </Button>
              </>
            )}
          </div>
        )}
      </div>

      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard label="Active clients" value={t.active_clients} />
        <StatCard label="Upcoming sessions" value={upcoming.length} />
        <StatCard label="Completed this month" value={completedThisMonth} />
        <StatCard label="Sessions remaining" value={activePkgs.reduce((a, p) => a + p.sessions_remaining, 0)} sub="across active packages" />
      </div>

      <Tabs<Tab>
        value={tab}
        onChange={setTab}
        tabs={[
          { id: 'upcoming', label: 'Upcoming', count: upcoming.length },
          { id: 'clients', label: 'Clients & packages', count: t.packages.length },
          { id: 'history', label: 'History', count: history.length },
        ]}
      />

      <div className="mt-6">
        {tab === 'upcoming' &&
          (upcoming.length === 0 ? (
            <div className="card">
              <EmptyState title="No upcoming sessions" description="Scheduled sessions for this trainer will appear here." />
            </div>
          ) : (
            sessionRows(upcoming)
          ))}

        {tab === 'clients' &&
          (t.packages.length === 0 ? (
            <div className="card">
              <EmptyState title="No packages assigned" description="Assign this trainer when selling a PT package to a member." />
            </div>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>Member</Th>
                  <Th>Package</Th>
                  <Th className="w-56">Progress</Th>
                  <Th className="text-right">Price</Th>
                  <Th>Status</Th>
                </tr>
              </thead>
              <tbody>
                {t.packages.map((p) => (
                  <tr key={p.id}>
                    <Td>
                      <Link to={`/members/${p.member_id}`} className="font-medium text-slate-900 hover:bg-signal">
                        {p.member_name}
                      </Link>
                    </Td>
                    <Td>
                      <div>{p.name}</div>
                      <div className="text-xs text-slate-400">bought {formatDate(p.purchased_at)}</div>
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
                    <Td className="text-right tabular-nums">{formatMoney(p.price, tenant.currency)}</Td>
                    <Td>
                      <Badge tone={statusTone(p.status)}>{titleCase(p.status)}</Badge>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          ))}

        {tab === 'history' &&
          (history.length === 0 ? (
            <div className="card">
              <EmptyState title="No past sessions" />
            </div>
          ) : (
            sessionRows(history)
          ))}
      </div>

      <TrainerFormModal open={editing} onClose={() => setEditing(false)} trainer={t} />
      <UserFormModal open={loginOpen} onClose={() => setLoginOpen(false)} trainer={t} />
      <SessionFormModal open={editSession !== null} onClose={() => setEditSession(null)} session={editSession ?? undefined} />
      <ConfirmDialog
        open={deleting}
        onClose={() => setDeleting(false)}
        title="Delete trainer?"
        description={deleteError || `Remove ${t.full_name}? Trainers with packages or sessions cannot be deleted; mark them inactive instead.`}
        loading={del.isPending}
        onConfirm={async () => {
          try {
            await del.mutateAsync(t.id)
            navigate('/trainers', { replace: true })
          } catch (e) {
            setDeleteError(errorMessage(e))
          }
        }}
      />
    </div>
  )
}
