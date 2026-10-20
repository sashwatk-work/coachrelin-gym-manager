import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import type { PTPackage, PTSession, Subscription } from '@/api/types'
import { useDeleteMember, useDeletePayment, useMember, useSetSessionStatus, useUpdateSubscription } from '@/api/hooks'
import { useAuth, useTenant } from '@/lib/auth'
import { can } from '@/lib/rbac'
import { addDays, errorMessage, formatDate, formatDateTime, formatMoney, titleCase } from '@/lib/format'
import {
  ActionMenu,
  Avatar,
  Badge,
  Button,
  ConfirmDialog,
  EmptyState,
  ErrorState,
  Loading,
  MenuItem,
  ProgressBar,
  StatCard,
  Table,
  Tabs,
  Td,
  Th,
  statusTone,
} from '@/components/ui'
import { downloadBillPdf, openWhatsAppBill, paymentReceipt } from '@/lib/bill'
import { MemberFormModal } from '@/components/forms/MemberForm'
import { SubscriptionFormModal } from '@/components/forms/SubscriptionForm'
import { PaymentFormModal } from '@/components/forms/PaymentForm'
import { PackageFormModal } from '@/components/forms/PackageForm'
import { SessionFormModal } from '@/components/forms/SessionForm'
import { MembershipBadge } from './MembersPage'
import { SessionStatusMenu } from './TrainingPage'

type Tab = 'overview' | 'membership' | 'training' | 'payments'

export default function MemberDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const tenant = useTenant()
  const { user } = useAuth()
  const q = useMember(id)
  const canWrite = can(user?.roles, 'membersWrite')
  const canPay = can(user?.roles, 'payments')
  const canSell = can(user?.roles, 'sellPT')
  const canDelete = can(user?.roles, 'deleteMembers')
  const canSchedule = can(user?.roles, 'scheduleSessions')
  const showPayments = can(user?.roles, 'viewPayments')
  const del = useDeleteMember()
  const updateSub = useUpdateSubscription()
  const delPayment = useDeletePayment()
  const setStatus = useSetSessionStatus()

  const [tab, setTab] = useState<Tab>('overview')
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [addSub, setAddSub] = useState(false)
  const [payFor, setPayFor] = useState<{ subscription_id?: string; pt_package_id?: string } | null>(null)
  const [pkgModal, setPkgModal] = useState<{ open: boolean; pkg?: PTPackage }>({ open: false })
  const [sessionModal, setSessionModal] = useState<{ open: boolean; pkg?: PTPackage; session?: PTSession }>({ open: false })
  const [cancelSub, setCancelSub] = useState<Subscription | null>(null)

  if (q.isLoading) return <Loading />
  if (q.isError || !q.data) return <ErrorState message={errorMessage(q.error)} onRetry={() => q.refetch()} />
  const m = q.data
  const money = (n: number) => formatMoney(n, tenant.currency)

  const currentSub = m.subscriptions.find((s) => s.status === 'active') ?? m.subscriptions[0]
  const suggestedStart = currentSub && currentSub.status === 'active' ? addDays(currentSub.end_date, 1) : undefined
  const activePkgs = m.packages.filter((p) => p.status === 'active')
  const upcoming = m.sessions.filter((s) => s.status === 'scheduled' && new Date(s.scheduled_at) >= new Date()).sort((a, b) => a.scheduled_at.localeCompare(b.scheduled_at))
  const totalPaid = m.payments.reduce((a, p) => a + p.amount, 0)

  return (
    <div>
      <nav className="mb-4 text-sm text-slate-500">
        <Link to="/members" className="hover:text-slate-800">
          Members
        </Link>
        <span className="mx-2">/</span>
        <span className="text-slate-800">{m.full_name}</span>
      </nav>

      <div className="card mb-6 flex flex-wrap items-center gap-5 px-6 py-5">
        <Avatar name={m.full_name} size="lg" />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-xl font-semibold tracking-tight">{m.full_name}</h1>
            <Badge tone={statusTone(m.status)}>{titleCase(m.status)}</Badge>
            <MembershipBadge status={m.membership_status} endDate={m.membership_end_date} />
          </div>
          <p className="mt-1 text-sm text-slate-500">
            {[m.phone, m.email, `Member since ${formatDate(m.joined_at)}`].filter(Boolean).join(' · ')}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {canPay && (
            <Button variant="secondary" onClick={() => setPayFor({})}>
              Record payment
            </Button>
          )}
          {canSell && (
            <Button variant="secondary" onClick={() => setPkgModal({ open: true })}>
              Buy PT package
            </Button>
          )}
          {canWrite && (
            <Button onClick={() => setAddSub(true)}>{m.membership_status === 'active' || m.membership_status === 'expiring' ? 'Renew membership' : 'Add membership'}</Button>
          )}
          {canWrite && (
            <Button variant="ghost" onClick={() => setEditing(true)}>
              Edit
            </Button>
          )}
        </div>
      </div>

      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard label="Current plan" value={m.current_plan_name ?? '—'} sub={m.membership_end_date ? `Ends ${formatDate(m.membership_end_date)}` : 'No active membership'} />
        {showPayments && (
          <StatCard label="Outstanding dues" value={money(m.outstanding_dues)} tone={m.outstanding_dues > 0 ? 'danger' : 'default'} sub={`${money(totalPaid)} paid in total`} />
        )}
        <StatCard
          label="PT sessions left"
          value={activePkgs.reduce((a, p) => a + p.sessions_remaining, 0)}
          sub={activePkgs.length ? `${activePkgs.length} active package${activePkgs.length > 1 ? 's' : ''}` : 'No active package'}
        />
        <StatCard label="Next session" value={upcoming[0] ? formatDateTime(upcoming[0].scheduled_at) : '—'} sub={upcoming[0] ? `with ${upcoming[0].trainer_name}` : 'Nothing scheduled'} />
      </div>

      <Tabs<Tab>
        value={tab}
        onChange={setTab}
        tabs={[
          { id: 'overview', label: 'Profile' },
          { id: 'membership', label: 'Membership', count: m.subscriptions.length },
          { id: 'training', label: 'Personal training', count: m.packages.length },
          ...(showPayments ? [{ id: 'payments' as const, label: 'Payments', count: m.payments.length }] : []),
        ]}
      />

      <div className="mt-6">
        {tab === 'overview' && (
          <div className="grid gap-6 lg:grid-cols-3">
            <section className="card lg:col-span-2">
              <h2 className="border-b border-slate-100 px-5 py-3.5 text-sm font-semibold">Details</h2>
              <dl className="grid grid-cols-1 gap-x-6 gap-y-4 px-5 py-5 sm:grid-cols-2">
                <Item label="Phone" value={m.phone} />
                <Item label="Email" value={m.email} />
                <Item label="Date of birth" value={m.date_of_birth ? formatDate(m.date_of_birth) : null} />
                <Item label="Gender" value={m.gender ? titleCase(m.gender) : null} />
                <Item label="Joined" value={formatDate(m.joined_at)} />
                <Item label="Emergency contact" value={m.emergency_contact} />
                <div className="sm:col-span-2">
                  <dt className="text-xs font-medium uppercase tracking-wide text-slate-400">Notes</dt>
                  <dd className="mt-1 whitespace-pre-wrap text-sm text-slate-700">{m.notes || <span className="text-slate-400">—</span>}</dd>
                </div>
              </dl>
            </section>
            <section className="card">
              <h2 className="border-b border-slate-100 px-5 py-3.5 text-sm font-semibold">Upcoming sessions</h2>
              {upcoming.length === 0 ? (
                <EmptyState title="No upcoming sessions" />
              ) : (
                <ul className="divide-y divide-slate-100">
                  {upcoming.slice(0, 6).map((s) => (
                    <li key={s.id} className="px-5 py-3">
                      <p className="text-sm font-medium text-slate-800">{formatDateTime(s.scheduled_at)}</p>
                      <p className="text-xs text-slate-500">
                        {s.package_name} · {s.trainer_name}
                      </p>
                    </li>
                  ))}
                </ul>
              )}
              {canDelete && (
                <div className="border-t border-slate-100 px-5 py-3">
                  <Button variant="ghost" size="sm" onClick={() => setDeleting(true)} className="text-error hover:bg-[#f3c4bf]">
                    Delete member
                  </Button>
                </div>
              )}
            </section>
          </div>
        )}

        {tab === 'membership' && (
          <div className="space-y-4">
            {canWrite && (
              <div className="flex justify-end">
                <Button onClick={() => setAddSub(true)}>Add membership</Button>
              </div>
            )}
            {m.subscriptions.length === 0 ? (
              <div className="card">
                <EmptyState title="No memberships yet" description="Add a membership to start tracking this member's access period and dues." action={canWrite ? <Button onClick={() => setAddSub(true)}>Add membership</Button> : undefined} />
              </div>
            ) : (
              <Table>
                <thead>
                  <tr>
                    <Th>Plan</Th>
                    <Th>Period</Th>
                    <Th>Status</Th>
                    <Th className="text-right">Price</Th>
                    <Th className="text-right">Paid</Th>
                    <Th className="text-right">Due</Th>
                    <Th></Th>
                  </tr>
                </thead>
                <tbody>
                  {m.subscriptions.map((s) => {
                    const due = Math.max(0, s.price - s.paid)
                    return (
                      <tr key={s.id}>
                        <Td className="font-medium text-slate-900">{s.plan_name}</Td>
                        <Td className="whitespace-nowrap">
                          {formatDate(s.start_date)} – {formatDate(s.end_date)}
                        </Td>
                        <Td>
                          <Badge tone={statusTone(s.status)}>{titleCase(s.status)}</Badge>
                        </Td>
                        <Td className="text-right tabular-nums">{money(s.price)}</Td>
                        <Td className="text-right tabular-nums">{money(s.paid)}</Td>
                        <Td className="text-right tabular-nums">{due > 0 ? <span className="font-medium text-error">{money(due)}</span> : <span className="text-ink">Paid</span>}</Td>
                        <Td className="text-right">
                          <div className="flex justify-end gap-1">
                            {canPay && due > 0 && s.status !== 'cancelled' && (
                              <Button size="sm" variant="secondary" onClick={() => setPayFor({ subscription_id: s.id })}>
                                Collect
                              </Button>
                            )}
                            {canWrite && s.status === 'active' && (
                              <Button size="sm" variant="ghost" onClick={() => setCancelSub(s)}>
                                Cancel
                              </Button>
                            )}
                            {canWrite && s.status === 'cancelled' && (
                              <Button size="sm" variant="ghost" onClick={() => updateSub.mutate({ id: s.id, status: 'active' })}>
                                Reactivate
                              </Button>
                            )}
                          </div>
                        </Td>
                      </tr>
                    )
                  })}
                </tbody>
              </Table>
            )}
          </div>
        )}

        {tab === 'training' && (
          <div className="space-y-6">
            <div className="flex justify-end gap-2">
              {canSchedule && activePkgs.length > 0 && (
                <Button variant="secondary" onClick={() => setSessionModal({ open: true, pkg: activePkgs[0] })}>
                  Schedule session
                </Button>
              )}
              {canSell && <Button onClick={() => setPkgModal({ open: true })}>Buy PT package</Button>}
            </div>
            {m.packages.length === 0 ? (
              <div className="card">
                <EmptyState title="No PT packages" description="Sell a package from Plans → Personal training. Sessions and price can be customised for this member." action={canSell ? <Button onClick={() => setPkgModal({ open: true })}>Buy PT package</Button> : undefined} />
              </div>
            ) : (
              <div className="grid gap-4 md:grid-cols-2">
                {m.packages.map((p) => {
                  const due = Math.max(0, p.price - p.paid)
                  return (
                    <div key={p.id} className="card p-5">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="font-medium text-slate-900">{p.name}</p>
                          <p className="text-sm text-slate-500">
                            with{' '}
                            <Link to={`/trainers/${p.trainer_id}`} className="text-slate-700 hover:bg-signal">
                              {p.trainer_name}
                            </Link>{' '}
                            · bought {formatDate(p.purchased_at)}
                            {p.expires_at ? ` · expires ${formatDate(p.expires_at)}` : ''}
                          </p>
                        </div>
                        <Badge tone={statusTone(p.status)}>{titleCase(p.status)}</Badge>
                      </div>
                      <div className="mt-4">
                        <div className="mb-1 flex justify-between text-xs text-slate-500">
                          <span>
                            {p.sessions_used} of {p.total_sessions} used
                          </span>
                          <span>
                            {p.sessions_remaining} left{p.sessions_scheduled ? ` · ${p.sessions_scheduled} scheduled` : ''}
                          </span>
                        </div>
                        <ProgressBar value={p.sessions_used} max={p.total_sessions} />
                      </div>
                      <div className="mt-4 flex flex-wrap items-center justify-between gap-2 text-sm">
                        <span className="tabular-nums text-slate-600">
                          {money(p.price)} · {due > 0 ? <span className="font-medium text-error">{money(due)} due</span> : <span className="text-ink">Paid</span>}
                        </span>
                        <div className="flex gap-1">
                          {canPay && due > 0 && p.status !== 'cancelled' && (
                            <Button size="sm" variant="secondary" onClick={() => setPayFor({ pt_package_id: p.id })}>
                              Collect
                            </Button>
                          )}
                          {canSchedule && p.status === 'active' && (
                            <Button size="sm" variant="secondary" onClick={() => setSessionModal({ open: true, pkg: p })}>
                              Schedule
                            </Button>
                          )}
                          {canSell && (
                            <Button size="sm" variant="ghost" onClick={() => setPkgModal({ open: true, pkg: p })}>
                              Edit
                            </Button>
                          )}
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}

            <section>
              <h2 className="mb-3 text-sm font-semibold text-slate-700">Session history</h2>
              {m.sessions.length === 0 ? (
                <div className="card">
                  <EmptyState title="No sessions yet" />
                </div>
              ) : (
                <Table>
                  <thead>
                    <tr>
                      <Th>When</Th>
                      <Th>Package</Th>
                      <Th>Trainer</Th>
                      <Th>Duration</Th>
                      <Th>Status</Th>
                      <Th></Th>
                    </tr>
                  </thead>
                  <tbody>
                    {m.sessions.map((s) => (
                      <tr key={s.id}>
                        <Td className="whitespace-nowrap font-medium text-slate-900">{formatDateTime(s.scheduled_at)}</Td>
                        <Td>{s.package_name}</Td>
                        <Td>{s.trainer_name}</Td>
                        <Td>{s.duration_minutes} min</Td>
                        <Td>
                          <Badge tone={statusTone(s.status)}>{titleCase(s.status)}</Badge>
                        </Td>
                        <Td className="text-right">
                          <SessionStatusMenu
                            session={s}
                            onSet={(status) => setStatus.mutate({ id: s.id, status })}
                            onEdit={() => setSessionModal({ open: true, session: s })}
                          />
                        </Td>
                      </tr>
                    ))}
                  </tbody>
                </Table>
              )}
            </section>
          </div>
        )}

        {tab === 'payments' && (
          <div className="space-y-4">
            <div className="flex justify-end">
              <Button onClick={() => setPayFor({})}>Record payment</Button>
            </div>
            {m.payments.length === 0 ? (
              <div className="card">
                <EmptyState title="No payments recorded" action={<Button onClick={() => setPayFor({})}>Record payment</Button>} />
              </div>
            ) : (
              <Table>
                <thead>
                  <tr>
                    <Th>Date</Th>
                    <Th>For</Th>
                    <Th>Method</Th>
                    <Th>Notes</Th>
                    <Th className="text-right">Amount</Th>
                    <Th></Th>
                  </tr>
                </thead>
                <tbody>
                  {m.payments.map((p) => (
                    <tr key={p.id}>
                      <Td className="whitespace-nowrap">{formatDateTime(p.paid_at)}</Td>
                      <Td>{p.reference_label}</Td>
                      <Td>{titleCase(p.method)}</Td>
                      <Td className="max-w-xs truncate text-slate-500">{p.notes || '—'}</Td>
                      <Td className="text-right font-medium tabular-nums text-ink">{money(p.amount)}</Td>
                      <Td className="text-right">
                        <ActionMenu>
                          <MenuItem onClick={() => void downloadBillPdf(paymentReceipt(p, tenant, m.phone))}>
                            Download receipt
                          </MenuItem>
                          <MenuItem
                            onClick={() => {
                              if (!openWhatsAppBill(m.phone, paymentReceipt(p, tenant, m.phone))) {
                                window.alert('Add a phone number for this member to send the receipt on WhatsApp.')
                              }
                            }}
                          >
                            WhatsApp receipt
                          </MenuItem>
                          {can(user?.roles, 'deletePayments') && (
                            <MenuItem
                              danger
                              onClick={() => {
                                if (confirm('Delete this payment record?')) delPayment.mutate(p.id)
                              }}
                            >
                              Delete
                            </MenuItem>
                          )}
                        </ActionMenu>
                      </Td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            )}
          </div>
        )}
      </div>

      <MemberFormModal open={editing} onClose={() => setEditing(false)} member={m} />
      <SubscriptionFormModal open={addSub} onClose={() => setAddSub(false)} memberId={m.id} memberName={m.full_name} suggestedStart={suggestedStart} />
      <PaymentFormModal
        open={payFor !== null}
        onClose={() => setPayFor(null)}
        memberId={m.id}
        memberName={m.full_name}
        subscriptions={m.subscriptions}
        packages={m.packages}
        outstandingDues={m.outstanding_dues}
        defaultReference={payFor ?? undefined}
      />
      <PackageFormModal open={pkgModal.open} onClose={() => setPkgModal({ open: false })} pkg={pkgModal.pkg} memberId={m.id} memberName={m.full_name} />
      <SessionFormModal open={sessionModal.open} onClose={() => setSessionModal({ open: false })} pkg={sessionModal.pkg} session={sessionModal.session} memberId={m.id} />
      <ConfirmDialog
        open={cancelSub !== null}
        onClose={() => setCancelSub(null)}
        title="Cancel membership?"
        description={`This marks the ${cancelSub?.plan_name ?? ''} membership as cancelled. Payments are kept.`}
        confirmLabel="Cancel membership"
        loading={updateSub.isPending}
        onConfirm={async () => {
          if (cancelSub) await updateSub.mutateAsync({ id: cancelSub.id, status: 'cancelled' })
          setCancelSub(null)
        }}
      />
      <ConfirmDialog
        open={deleting}
        onClose={() => setDeleting(false)}
        title="Delete member?"
        description={`This permanently removes ${m.full_name} along with their memberships, payments, PT packages and sessions.`}
        loading={del.isPending}
        onConfirm={async () => {
          await del.mutateAsync(m.id)
          navigate('/members', { replace: true })
        }}
      />
    </div>
  )
}

function Item({ label, value }: { label: string; value: string | null | undefined }) {
  return (
    <div>
      <dt className="text-xs font-medium uppercase tracking-wide text-slate-400">{label}</dt>
      <dd className="mt-1 text-sm text-slate-800">{value || <span className="text-slate-400">—</span>}</dd>
    </div>
  )
}
