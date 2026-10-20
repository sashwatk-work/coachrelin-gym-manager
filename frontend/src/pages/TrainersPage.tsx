import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { Trainer } from '@/api/types'
import { useTrainers, useUsers } from '@/api/hooks'
import { useAuth } from '@/lib/auth'
import { can } from '@/lib/rbac'
import { errorMessage } from '@/lib/format'
import { Avatar, Badge, Button, EmptyState, ErrorState, Loading, PageHeader, Select } from '@/components/ui'
import { TrainerFormModal } from '@/components/forms/TrainerForm'
import { UserFormModal, loginForTrainer } from '@/components/forms/UserForm'

export default function TrainersPage() {
  const { user } = useAuth()
  const canManage = can(user?.roles, 'manageTrainers')
  const canCreateLogin = can(user?.roles, 'manageUsers')
  const [status, setStatus] = useState('')
  const [modal, setModal] = useState<{ open: boolean; trainer?: Trainer }>({ open: false })
  const [loginFor, setLoginFor] = useState<Trainer | undefined>()
  const q = useTrainers({ status: status || undefined, page_size: 200 })
  const users = useUsers({ enabled: canCreateLogin })

  return (
    <div>
      <PageHeader
        title="Trainers"
        subtitle="Personal trainers who run PT packages and sessions."
        actions={
          <>
            <Select value={status} onChange={(e) => setStatus(e.target.value)} className="w-auto">
              <option value="">All</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </Select>
            {canManage && <Button onClick={() => setModal({ open: true })}>Add trainer</Button>}
          </>
        }
      />

      {q.isLoading ? (
        <Loading />
      ) : q.isError ? (
        <ErrorState message={errorMessage(q.error)} onRetry={() => q.refetch()} />
      ) : q.data!.items.length === 0 ? (
        <div className="card">
          <EmptyState title="No trainers yet" description="Add the trainers who will deliver personal-training sessions." action={canManage ? <Button onClick={() => setModal({ open: true })}>Add trainer</Button> : undefined} />
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {q.data!.items.map((t) => {
            const login = loginForTrainer(users.data?.items, t.id)
            return (
              <article key={t.id} className={`card p-5 transition-shadow hover:shadow-md ${!t.active ? 'opacity-70' : ''}`}>
                <Link to={`/trainers/${t.id}`} className="group block">
                  <div className="flex items-start gap-4">
                    <Avatar name={t.full_name} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-2">
                        <h2 className="truncate font-semibold text-slate-900 group-hover:bg-signal">{t.full_name}</h2>
                        <Badge tone={t.active ? 'green' : 'slate'}>{t.active ? 'Active' : 'Inactive'}</Badge>
                      </div>
                      <p className="truncate text-sm text-slate-500">{t.phone ?? t.email ?? '—'}</p>
                    </div>
                  </div>
                  {t.specialties.length > 0 && (
                    <div className="mt-3 flex flex-wrap gap-1">
                      {t.specialties.map((s) => (
                        <span key={s} className="rounded-md bg-slate-100 px-2 py-0.5 text-xs text-slate-600">
                          {s}
                        </span>
                      ))}
                    </div>
                  )}
                  <dl className="mt-4 grid grid-cols-2 gap-3 border-t border-slate-100 pt-4 text-sm">
                    <div>
                      <dt className="text-xs uppercase tracking-wide text-slate-400">Active clients</dt>
                      <dd className="mt-0.5 text-lg font-semibold tabular-nums">{t.active_clients}</dd>
                    </div>
                    <div>
                      <dt className="text-xs uppercase tracking-wide text-slate-400">Upcoming sessions</dt>
                      <dd className="mt-0.5 text-lg font-semibold tabular-nums">{t.upcoming_sessions}</dd>
                    </div>
                  </dl>
                </Link>
                {canCreateLogin && (
                  <div className="mt-4 flex items-center justify-between gap-2 border-t border-slate-100 pt-3">
                    {login ? (
                      <p className="truncate text-xs text-slate-500">Signs in as {login.email}</p>
                    ) : (
                      <>
                        <p className="text-xs text-slate-400">No login</p>
                        <Button size="sm" variant="secondary" onClick={() => setLoginFor(t)}>
                          Create login
                        </Button>
                      </>
                    )}
                  </div>
                )}
              </article>
            )
          })}
        </div>
      )}

      <TrainerFormModal open={modal.open} onClose={() => setModal({ open: false })} trainer={modal.trainer} />
      <UserFormModal open={!!loginFor} onClose={() => setLoginFor(undefined)} trainer={loginFor} />
    </div>
  )
}
