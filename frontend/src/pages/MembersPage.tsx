import { useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useMembers } from '@/api/hooks'
import { useAuth, useTenant } from '@/lib/auth'
import { can } from '@/lib/rbac'
import { errorMessage, formatDate, formatMoney, titleCase } from '@/lib/format'
import {
  Avatar,
  Badge,
  Button,
  EmptyState,
  ErrorState,
  Loading,
  PageHeader,
  Pagination,
  SearchInput,
  Select,
  Table,
  Td,
  Th,
  statusTone,
} from '@/components/ui'
import { MemberFormModal } from '@/components/forms/MemberForm'

const PAGE_SIZE = 25

function useDebounced<T>(value: T, ms = 250): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

export function MembershipBadge({ status, endDate }: { status: string; endDate: string | null }) {
  const label =
    status === 'none'
      ? 'No membership'
      : status === 'expired'
        ? `Expired ${formatDate(endDate)}`
        : status === 'expiring'
          ? `Ends ${formatDate(endDate)}`
          : `Until ${formatDate(endDate)}`
  return <Badge tone={statusTone(status)}>{label}</Badge>
}

export default function MembersPage() {
  const tenant = useTenant()
  const { user } = useAuth()
  const navigate = useNavigate()
  const canWrite = can(user?.roles, 'membersWrite')
  const showDues = can(user?.roles, 'viewPayments')
  const [params, setParams] = useSearchParams()
  const [q, setQ] = useState(params.get('q') ?? '')
  const dq = useDebounced(q)
  const status = params.get('status') ?? ''
  const membership = params.get('membership') ?? ''
  const page = Number(params.get('page') ?? '1') || 1
  const [adding, setAdding] = useState(false)

  const setParam = (k: string, v: string) => {
    const next = new URLSearchParams(params)
    if (v) next.set(k, v)
    else next.delete(k)
    if (k !== 'page') next.delete('page')
    setParams(next, { replace: true })
  }

  useEffect(() => {
    if ((params.get('q') ?? '') !== dq) setParam('q', dq)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dq])

  const list = useMembers({ q: dq, status, membership, page, page_size: PAGE_SIZE })

  return (
    <div>
      <PageHeader
        title="Members"
        subtitle={list.data ? `${list.data.total} member${list.data.total === 1 ? '' : 's'}` : undefined}
        actions={canWrite ? <Button onClick={() => setAdding(true)}>Add member</Button> : undefined}
      />

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <SearchInput value={q} onChange={setQ} placeholder="Search name, phone, email…" className="w-full sm:w-72" />
        <Select value={membership} onChange={(e) => setParam('membership', e.target.value)} className="w-auto">
          <option value="">All memberships</option>
          <option value="active">Active</option>
          <option value="expiring">Expiring soon</option>
          <option value="expired">Expired</option>
          <option value="none">No membership</option>
        </Select>
        <Select value={status} onChange={(e) => setParam('status', e.target.value)} className="w-auto">
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="frozen">Frozen</option>
          <option value="inactive">Inactive</option>
        </Select>
        {(q || status || membership) && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setQ('')
              setParams({}, { replace: true })
            }}
          >
            Clear
          </Button>
        )}
      </div>

      {list.isLoading ? (
        <Loading />
      ) : list.isError ? (
        <ErrorState message={errorMessage(list.error)} onRetry={() => list.refetch()} />
      ) : list.data && list.data.items.length === 0 ? (
        <div className="card">
          <EmptyState
            title={q || status || membership ? 'No members match these filters' : 'No members yet'}
            description={q || status || membership ? 'Try a different search or clear the filters.' : 'Add your first member to get started.'}
            action={!q && !status && !membership && canWrite ? <Button onClick={() => setAdding(true)}>Add member</Button> : undefined}
          />
        </div>
      ) : (
        <>
          <Table>
            <thead>
              <tr>
                <Th>Member</Th>
                <Th>Contact</Th>
                <Th>Plan</Th>
                <Th>Membership</Th>
                <Th>Status</Th>
                {showDues && <Th className="text-right">Dues</Th>}
                <Th>Joined</Th>
              </tr>
            </thead>
            <tbody>
              {list.data!.items.map((m) => (
                <tr key={m.id} className="cursor-pointer hover:bg-slate-50" onClick={() => navigate(`/members/${m.id}`)}>
                  <Td>
                    <div className="flex items-center gap-3">
                      <Avatar name={m.full_name} size="sm" />
                      <Link to={`/members/${m.id}`} className="font-medium text-slate-900 hover:bg-signal" onClick={(e) => e.stopPropagation()}>
                        {m.full_name}
                      </Link>
                    </div>
                  </Td>
                  <Td>
                    <div className="text-slate-700">{m.phone ?? '—'}</div>
                    {m.email && <div className="text-xs text-slate-400">{m.email}</div>}
                  </Td>
                  <Td>{m.current_plan_name ?? <span className="text-slate-400">—</span>}</Td>
                  <Td>
                    <MembershipBadge status={m.membership_status} endDate={m.membership_end_date} />
                  </Td>
                  <Td>
                    <Badge tone={statusTone(m.status)}>{titleCase(m.status)}</Badge>
                  </Td>
                  {showDues && (
                    <Td className="text-right tabular-nums">
                      {m.outstanding_dues > 0 ? (
                        <span className="font-medium text-error">{formatMoney(m.outstanding_dues, tenant.currency)}</span>
                      ) : (
                        <span className="text-slate-400">—</span>
                      )}
                    </Td>
                  )}
                  <Td className="whitespace-nowrap text-slate-500">{formatDate(m.joined_at)}</Td>
                </tr>
              ))}
            </tbody>
          </Table>
          <Pagination page={page} pageSize={PAGE_SIZE} total={list.data!.total} onChange={(p) => setParam('page', String(p))} />
        </>
      )}

      <MemberFormModal open={adding} onClose={() => setAdding(false)} onSaved={(m) => navigate(`/members/${m.id}`)} />
    </div>
  )
}
