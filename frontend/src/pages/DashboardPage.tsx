import { Link } from 'react-router-dom'
import { useDashboard } from '@/api/hooks'
import { useAuth, useTenant } from '@/lib/auth'
import { isTrainerOnly } from '@/lib/rbac'
import { errorMessage, formatDate, formatMoney, formatTime, titleCase } from '@/lib/format'
import { Avatar, Badge, EmptyState, ErrorState, Loading, PageHeader, StatCard, statusTone } from '@/components/ui'

function greeting() {
  const h = new Date().getHours()
  if (h < 12) return 'Good morning'
  if (h < 17) return 'Good afternoon'
  return 'Good evening'
}

export default function DashboardPage() {
  const { user } = useAuth()
  const tenant = useTenant()
  const q = useDashboard()
  const isTrainer = isTrainerOnly(user?.roles)

  if (q.isLoading) return <Loading />
  if (q.isError || !q.data) return <ErrorState message={errorMessage(q.error)} onRetry={() => q.refetch()} />
  const d = q.data
  const money = (n: number) => formatMoney(n, tenant.currency)

  const revenueDelta =
    d.revenue_last_month > 0 ? Math.round(((d.revenue_this_month - d.revenue_last_month) / d.revenue_last_month) * 100) : null
  const maxTrend = Math.max(1, ...d.revenue_trend.map((p) => p.amount))

  return (
    <div>
      <PageHeader
        title={`${greeting()}, ${user?.full_name?.split(' ')[0] ?? ''}`}
        subtitle={`Here's what's happening at ${tenant.name} today, ${formatDate(d.today, { weekday: 'long', day: 'numeric', month: 'long' })}.`}
      />

      {isTrainer ? (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <StatCard label="Your clients" value={d.active_members} sub={`${d.total_members} assigned`} />
          <StatCard label="Sessions today" value={d.sessions_today} sub={`${d.sessions_this_week} this week`} />
          <StatCard label="Active packages" value={d.active_pt_packages} />
          <StatCard label="Completed this month" value={d.sessions_completed_this_month} />
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
            <StatCard label="Active members" value={d.active_members} sub={`${d.total_members} total · ${d.new_members_this_month} new this month`} />
            <StatCard
              label="Expiring in 7 days"
              value={d.expiring_soon}
              tone={d.expiring_soon > 0 ? 'warn' : 'default'}
              sub={`${d.expired_memberships} already expired`}
            />
            <StatCard
              label="Revenue this month"
              value={money(d.revenue_this_month)}
              tone="good"
              sub={
                revenueDelta === null
                  ? `${money(d.revenue_last_month)} last month`
                  : `${revenueDelta >= 0 ? '+' : ''}${revenueDelta}% vs last month`
              }
            />
            <StatCard label="Outstanding dues" value={money(d.outstanding_dues)} tone={d.outstanding_dues > 0 ? 'danger' : 'default'} sub="Unpaid memberships & PT packs" />
          </div>
          <div className="mt-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
            <StatCard label="PT sessions today" value={d.sessions_today} sub={`${d.sessions_this_week} this week`} />
            <StatCard label="Active PT packages" value={d.active_pt_packages} sub={`${d.sessions_completed_this_month} sessions completed this month`} />
            <StatCard label="Active trainers" value={d.active_trainers} />
            <StatCard label="Active memberships" value={d.active_memberships} />
          </div>
        </>
      )}

      <div className="mt-6 grid gap-6 lg:grid-cols-3">
        {/* Today's sessions */}
        <section className="card lg:col-span-1">
          <header className="flex items-center justify-between border-b border-slate-100 px-5 py-3.5">
            <h2 className="font-display text-xl font-semibold uppercase tracking-tight">Today's PT sessions</h2>
            <Link to="/training" className="console-link">
              View all
            </Link>
          </header>
          {d.today_sessions.length === 0 ? (
            <EmptyState title="No sessions today" description="Scheduled sessions will show up here." />
          ) : (
            <ul className="divide-y divide-slate-100">
              {d.today_sessions.map((s) => (
                <li key={s.id} className="flex items-center gap-3 px-5 py-3">
                  <div className="w-16 shrink-0 text-sm font-medium tabular-nums text-slate-900">{formatTime(s.scheduled_at)}</div>
                  <div className="min-w-0 flex-1">
                    <Link to={`/members/${s.member_id}`} className="block truncate text-sm font-medium text-slate-800 hover:bg-signal">
                      {s.member_name}
                    </Link>
                    <p className="truncate text-xs text-slate-500">with {s.trainer_name}</p>
                  </div>
                  <Badge tone={statusTone(s.status)}>{titleCase(s.status)}</Badge>
                </li>
              ))}
            </ul>
          )}
        </section>

        {/* Expiring memberships */}
        {!isTrainer && <section className="card lg:col-span-1">
          <header className="flex items-center justify-between border-b border-slate-100 px-5 py-3.5">
            <h2 className="font-display text-xl font-semibold uppercase tracking-tight">Expiring soon</h2>
            <Link to="/members?membership=expiring" className="console-link">
              View all
            </Link>
          </header>
          {d.expiring_memberships.length === 0 ? (
            <EmptyState title="Nothing expiring this week" description="Memberships ending within 7 days appear here." />
          ) : (
            <ul className="divide-y divide-slate-100">
              {d.expiring_memberships.map((e) => (
                <li key={e.member_id} className="flex items-center gap-3 px-5 py-3">
                  <Avatar name={e.member_name} size="sm" />
                  <div className="min-w-0 flex-1">
                    <Link to={`/members/${e.member_id}`} className="block truncate text-sm font-medium text-slate-800 hover:bg-signal">
                      {e.member_name}
                    </Link>
                    <p className="truncate text-xs text-slate-500">
                      {e.plan_name} · ends {formatDate(e.end_date)}
                    </p>
                  </div>
                  <Badge tone={e.days_left <= 2 ? 'red' : 'amber'}>
                    {e.days_left === 0 ? 'Today' : e.days_left === 1 ? '1 day' : `${e.days_left} days`}
                  </Badge>
                </li>
              ))}
            </ul>
          )}
        </section>}

        {/* Revenue trend + recent payments */}
        {!isTrainer && <section className="card lg:col-span-1">
          <header className="flex items-center justify-between border-b border-slate-100 px-5 py-3.5">
            <h2 className="font-display text-xl font-semibold uppercase tracking-tight">Revenue, last 6 months</h2>
            <Link to="/payments" className="console-link">
              Payments
            </Link>
          </header>
          <div className="px-5 pb-2 pt-4">
            <div className="flex h-28 items-end gap-2">
              {d.revenue_trend.map((p) => (
                <div key={p.month} className="group flex flex-1 flex-col items-center justify-end gap-1" title={`${p.month}: ${money(p.amount)}`}>
                  <div
                    className="w-full bg-signal transition-colors group-hover:bg-ink"
                    style={{ height: `${Math.max(3, (p.amount / maxTrend) * 100)}%` }}
                  />
                  <span className="text-[10px] uppercase text-slate-400">
                    {new Date(Number(p.month.slice(0, 4)), Number(p.month.slice(5)) - 1, 1).toLocaleString(undefined, { month: 'short' })}
                  </span>
                </div>
              ))}
            </div>
          </div>
          <div className="border-t border-slate-100">
            <p className="px-5 pb-1 pt-3 text-[10px] font-extrabold uppercase tracking-label text-muted">Recent payments</p>
            {d.recent_payments.length === 0 ? (
              <p className="px-5 pb-4 text-sm text-slate-500">No payments yet.</p>
            ) : (
              <ul className="divide-y divide-slate-100">
                {d.recent_payments.slice(0, 5).map((p) => (
                  <li key={p.id} className="flex items-center justify-between gap-3 px-5 py-2.5">
                    <div className="min-w-0">
                      <Link to={`/members/${p.member_id}`} className="block truncate text-sm text-slate-800 hover:bg-signal">
                        {p.member_name}
                      </Link>
                      <p className="truncate text-xs text-slate-500">
                        {p.reference_label} · {titleCase(p.method)}
                      </p>
                    </div>
                    <span className="text-sm font-semibold tabular-nums text-ink">+{money(p.amount)}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>}
      </div>
    </div>
  )
}
