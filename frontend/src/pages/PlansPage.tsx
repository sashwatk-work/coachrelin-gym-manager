import { useState } from 'react'
import type { MembershipPlan, PTPlan } from '@/api/types'
import { useDeletePlan, useDeletePTPlan, usePlans, usePTPlans, useUpdatePlan, useUpdatePTPlan } from '@/api/hooks'
import { useAuth, useTenant } from '@/lib/auth'
import { can } from '@/lib/rbac'
import { errorMessage, formatMoney } from '@/lib/format'
import { Badge, Button, ConfirmDialog, EmptyState, ErrorState, Loading, PageHeader, Tabs } from '@/components/ui'
import { PlanFormModal } from '@/components/forms/PlanForm'
import { PTPlanFormModal } from '@/components/forms/PTPlanForm'

function durationLabel(days: number): string {
  if (days % 365 === 0) return days === 365 ? '1 year' : `${days / 365} years`
  if (days % 30 === 0) return days === 30 ? '1 month' : `${days / 30} months`
  if (days % 7 === 0) return days === 7 ? '1 week' : `${days / 7} weeks`
  return `${days} days`
}

type Tab = 'membership' | 'training'

export default function PlansPage() {
  const [tab, setTab] = useState<Tab>('membership')
  return (
    <div>
      <PageHeader
        title="Plans"
        subtitle="Membership tiers and personal-training packages members can buy. Prices can still be customised per member."
      />
      <Tabs
        tabs={[
          { id: 'membership', label: 'Membership' },
          { id: 'training', label: 'Personal training' },
        ]}
        value={tab}
        onChange={setTab}
      />
      <div className="mt-6">{tab === 'membership' ? <MembershipPlans /> : <PTPlans />}</div>
    </div>
  )
}

function MembershipPlans() {
  const tenant = useTenant()
  const { user } = useAuth()
  const canManage = can(user?.roles, 'managePlans')
  const q = usePlans(false)
  const update = useUpdatePlan()
  const del = useDeletePlan()
  const [modal, setModal] = useState<{ open: boolean; plan?: MembershipPlan }>({ open: false })
  const [deleting, setDeleting] = useState<MembershipPlan | null>(null)
  const [deleteError, setDeleteError] = useState('')
  const money = (n: number) => formatMoney(n, tenant.currency)

  return (
    <div>
      {canManage && (
        <div className="mb-4 flex justify-end">
          <Button onClick={() => setModal({ open: true })}>New membership plan</Button>
        </div>
      )}
      {q.isLoading ? (
        <Loading />
      ) : q.isError ? (
        <ErrorState message={errorMessage(q.error)} onRetry={() => q.refetch()} />
      ) : q.data!.length === 0 ? (
        <div className="card">
          <EmptyState
            title="No membership plans yet"
            description="Create your first plan, e.g. Monthly, Quarterly, Annual."
            action={canManage ? <Button onClick={() => setModal({ open: true })}>New membership plan</Button> : undefined}
          />
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {q.data!.map((p) => (
            <div key={p.id} className={`card flex flex-col p-5 ${!p.active ? 'opacity-70' : ''}`}>
              <div className="flex items-start justify-between gap-2">
                <div>
                  <h2 className="text-base font-semibold text-slate-900">{p.name}</h2>
                  <p className="text-sm text-slate-500">{durationLabel(p.duration_days)}</p>
                </div>
                <Badge tone={p.active ? 'green' : 'slate'}>{p.active ? 'Active' : 'Inactive'}</Badge>
              </div>
              <p className="mt-4 text-2xl font-semibold tabular-nums tracking-tight">{money(p.price)}</p>
              <p className="text-xs text-slate-400">≈ {money(Math.round((p.price / p.duration_days) * 30))} / month</p>
              {p.description && <p className="mt-3 text-sm text-slate-600">{p.description}</p>}
              <div className="mt-auto flex items-center justify-between pt-5 text-sm">
                <span className="text-slate-500">
                  {p.active_subscriptions} active member{p.active_subscriptions === 1 ? '' : 's'}
                </span>
                {canManage && (
                  <div className="flex gap-1">
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() =>
                        update.mutate({
                          id: p.id,
                          body: { name: p.name, duration_days: p.duration_days, price: p.price, description: p.description, active: !p.active },
                        })
                      }
                    >
                      {p.active ? 'Deactivate' : 'Activate'}
                    </Button>
                    <Button size="sm" variant="secondary" onClick={() => setModal({ open: true, plan: p })}>
                      Edit
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-slate-400 hover:text-error"
                      onClick={() => {
                        setDeleteError('')
                        setDeleting(p)
                      }}
                    >
                      Delete
                    </Button>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      <PlanFormModal open={modal.open} onClose={() => setModal({ open: false })} plan={modal.plan} />
      <ConfirmDialog
        open={deleting !== null}
        onClose={() => setDeleting(null)}
        title="Delete plan?"
        description={deleteError || `Delete "${deleting?.name}"? Plans that have ever been used by a membership cannot be deleted; deactivate them instead.`}
        loading={del.isPending}
        onConfirm={async () => {
          if (!deleting) return
          try {
            await del.mutateAsync(deleting.id)
            setDeleting(null)
          } catch (e) {
            setDeleteError(errorMessage(e))
          }
        }}
      />
    </div>
  )
}

function PTPlans() {
  const tenant = useTenant()
  const { user } = useAuth()
  const canManage = can(user?.roles, 'managePlans')
  const q = usePTPlans(false)
  const update = useUpdatePTPlan()
  const del = useDeletePTPlan()
  const [modal, setModal] = useState<{ open: boolean; plan?: PTPlan }>({ open: false })
  const [deleting, setDeleting] = useState<PTPlan | null>(null)
  const [deleteError, setDeleteError] = useState('')
  const money = (n: number) => formatMoney(n, tenant.currency)

  return (
    <div>
      {canManage && (
        <div className="mb-4 flex justify-end">
          <Button onClick={() => setModal({ open: true })}>New PT package</Button>
        </div>
      )}
      {q.isLoading ? (
        <Loading />
      ) : q.isError ? (
        <ErrorState message={errorMessage(q.error)} onRetry={() => q.refetch()} />
      ) : q.data!.length === 0 ? (
        <div className="card">
          <EmptyState
            title="No PT packages yet"
            description="Create reusable packs (e.g. 8 sessions, 12 sessions) then sell them to members. You can still customise sessions and price per member."
            action={canManage ? <Button onClick={() => setModal({ open: true })}>New PT package</Button> : undefined}
          />
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {q.data!.map((p) => (
            <div key={p.id} className={`card flex flex-col p-5 ${!p.active ? 'opacity-70' : ''}`}>
              <div className="flex items-start justify-between gap-2">
                <div>
                  <h2 className="text-base font-semibold text-slate-900">{p.name}</h2>
                  <p className="text-sm text-slate-500">
                    {p.sessions} session{p.sessions === 1 ? '' : 's'}
                    {p.validity_days ? ` · ${durationLabel(p.validity_days)}` : ' · no expiry'}
                  </p>
                </div>
                <Badge tone={p.active ? 'green' : 'slate'}>{p.active ? 'Active' : 'Inactive'}</Badge>
              </div>
              <p className="mt-4 text-2xl font-semibold tabular-nums tracking-tight">{money(p.price)}</p>
              <p className="text-xs text-slate-400">{money(p.price / p.sessions)} / session</p>
              {p.description && <p className="mt-3 text-sm text-slate-600">{p.description}</p>}
              <div className="mt-auto flex items-center justify-between pt-5 text-sm">
                <span className="text-slate-500">
                  {p.active_packages} active sale{p.active_packages === 1 ? '' : 's'}
                </span>
                {canManage && (
                <div className="flex gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      update.mutate({
                        id: p.id,
                        body: {
                          name: p.name,
                          sessions: p.sessions,
                          validity_days: p.validity_days,
                          price: p.price,
                          description: p.description,
                          active: !p.active,
                        },
                      })
                    }
                  >
                    {p.active ? 'Deactivate' : 'Activate'}
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => setModal({ open: true, plan: p })}>
                    Edit
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="text-slate-400 hover:text-error"
                    onClick={() => {
                      setDeleteError('')
                      setDeleting(p)
                    }}
                  >
                    Delete
                  </Button>
                </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      <PTPlanFormModal open={modal.open} onClose={() => setModal({ open: false })} plan={modal.plan} />
      <ConfirmDialog
        open={deleting !== null}
        onClose={() => setDeleting(null)}
        title="Delete PT package?"
        description={deleteError || `Delete "${deleting?.name}"? Packages already sold to members keep their own copy; this only removes it from the catalogue. If it is in use it cannot be deleted — deactivate it instead.`}
        loading={del.isPending}
        onConfirm={async () => {
          if (!deleting) return
          try {
            await del.mutateAsync(deleting.id)
            setDeleting(null)
          } catch (e) {
            setDeleteError(errorMessage(e))
          }
        }}
      />
    </div>
  )
}
