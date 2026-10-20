import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useDeletePayment, useMember, useMembers, usePayments } from '@/api/hooks'
import { useAuth, useTenant } from '@/lib/auth'
import { can } from '@/lib/rbac'
import { addDays, errorMessage, formatDateTime, formatMoney, titleCase, todayISO } from '@/lib/format'
import { ActionMenu, Button, EmptyState, ErrorState, Input, Loading, MenuItem, Modal, PageHeader, Pagination, SearchInput, Select, StatCard, Table, Td, Th } from '@/components/ui'
import { downloadBillPdf, paymentReceipt } from '@/lib/bill'
import { PaymentFormModal } from '@/components/forms/PaymentForm'

const PAGE_SIZE = 30
type Range = 'month' | '30d' | 'all' | 'custom'

function monthStart(): string {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-01`
}

export default function PaymentsPage() {
  const tenant = useTenant()
  const { user } = useAuth()
  const canDelete = can(user?.roles, 'deletePayments')
  const [range, setRange] = useState<Range>('month')
  const [from, setFrom] = useState(monthStart())
  const [to, setTo] = useState(todayISO())
  const [method, setMethod] = useState('')
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [picking, setPicking] = useState(false)
  const [memberFor, setMemberFor] = useState<{ id: string; name: string } | null>(null)
  const del = useDeletePayment()

  const dates =
    range === 'month'
      ? { from: monthStart(), to: todayISO() }
      : range === '30d'
        ? { from: addDays(todayISO(), -29), to: todayISO() }
        : range === 'custom'
          ? { from: from || undefined, to: to || undefined }
          : {}

  const list = usePayments({ ...dates, method: method || undefined, q: q || undefined, page, page_size: PAGE_SIZE })
  useEffect(() => setPage(1), [range, from, to, method, q])
  const money = (n: number) => formatMoney(n, tenant.currency)

  return (
    <div>
      <PageHeader title="Payments" subtitle="Every payment received, linked to the membership or PT package it covers." actions={<Button onClick={() => setPicking(true)}>Record payment</Button>} />

      <div className="mb-4 grid grid-cols-2 gap-4 sm:grid-cols-3">
        <StatCard label="Total in range" value={list.data ? money(list.data.total_amount) : '—'} tone="good" />
        <StatCard label="Payments" value={list.data?.total ?? '—'} />
        <StatCard label="Average" value={list.data && list.data.total > 0 ? money(list.data.total_amount / list.data.total) : '—'} />
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="inline-flex rounded-lg border border-slate-300 bg-white p-0.5 text-sm shadow-sm">
          {(['month', '30d', 'all', 'custom'] as Range[]).map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => setRange(r)}
              className={`rounded-md px-3 py-1.5 font-medium transition-colors ${range === r ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100'}`}
            >
              {r === 'month' ? 'This month' : r === '30d' ? 'Last 30 days' : r === 'all' ? 'All time' : 'Custom'}
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
        <Select value={method} onChange={(e) => setMethod(e.target.value)} className="w-auto">
          <option value="">All methods</option>
          <option value="upi">UPI</option>
          <option value="cash">Cash</option>
          <option value="card">Card</option>
          <option value="bank_transfer">Bank transfer</option>
          <option value="other">Other</option>
        </Select>
        <SearchInput value={q} onChange={setQ} placeholder="Member or note…" className="w-56" />
      </div>

      {list.isLoading ? (
        <Loading />
      ) : list.isError ? (
        <ErrorState message={errorMessage(list.error)} onRetry={() => list.refetch()} />
      ) : list.data!.items.length === 0 ? (
        <div className="card">
          <EmptyState title="No payments in this range" action={<Button onClick={() => setPicking(true)}>Record payment</Button>} />
        </div>
      ) : (
        <>
          <Table>
            <thead>
              <tr>
                <Th>Date</Th>
                <Th>Member</Th>
                <Th>For</Th>
                <Th>Method</Th>
                <Th>Notes</Th>
                <Th className="text-right">Amount</Th>
                <Th></Th>
              </tr>
            </thead>
            <tbody>
              {list.data!.items.map((p) => (
                <tr key={p.id}>
                  <Td className="whitespace-nowrap text-slate-600">{formatDateTime(p.paid_at)}</Td>
                  <Td>
                    <Link to={`/members/${p.member_id}`} className="font-medium text-slate-900 hover:bg-signal">
                      {p.member_name}
                    </Link>
                  </Td>
                  <Td>{p.reference_label}</Td>
                  <Td>{titleCase(p.method)}</Td>
                  <Td className="max-w-xs truncate text-slate-500">{p.notes || '—'}</Td>
                  <Td className="text-right font-medium tabular-nums text-ink">{money(p.amount)}</Td>
                  <Td className="text-right">
                    <ActionMenu>
                      <MenuItem onClick={() => void downloadBillPdf(paymentReceipt(p, tenant))}>Download receipt</MenuItem>
                      {canDelete && (
                        <MenuItem
                          danger
                          onClick={() => {
                            if (confirm('Delete this payment record?')) del.mutate(p.id)
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
          <Pagination page={page} pageSize={PAGE_SIZE} total={list.data!.total} onChange={setPage} />
        </>
      )}

      <MemberPicker
        open={picking}
        onClose={() => setPicking(false)}
        onPick={(id, name) => {
          setPicking(false)
          setMemberFor({ id, name })
        }}
      />
      {memberFor && <PaymentForMember member={memberFor} onClose={() => setMemberFor(null)} />}
    </div>
  )
}

/** Small member search to choose who is paying. */
function MemberPicker({ open, onClose, onPick }: { open: boolean; onClose: () => void; onPick: (id: string, name: string) => void }) {
  const [q, setQ] = useState('')
  const list = useMembers({ q, page_size: 10 })
  useEffect(() => {
    if (open) setQ('')
  }, [open])
  return (
    <Modal open={open} onClose={onClose} title="Who is paying?" size="sm">
      <SearchInput value={q} onChange={setQ} placeholder="Search members…" />
      <ul className="mt-3 max-h-72 divide-y divide-slate-100 overflow-y-auto rounded-lg border border-slate-200">
        {list.data?.items.map((m) => (
          <li key={m.id}>
            <button type="button" onClick={() => onPick(m.id, m.full_name)} className="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-slate-50">
              <span className="font-medium text-slate-800">{m.full_name}</span>
              <span className="text-xs text-slate-400">{m.phone ?? m.email ?? ''}</span>
            </button>
          </li>
        ))}
        {list.data?.items.length === 0 && <li className="px-3 py-4 text-center text-sm text-slate-500">No members found</li>}
      </ul>
    </Modal>
  )
}

/** Loads a member's open items so the payment can be linked. The modal opens
 * once they are available so the form can pre-select the item with dues. */
function PaymentForMember({ member, onClose }: { member: { id: string; name: string }; onClose: () => void }) {
  const detail = useMember(member.id)
  useEffect(() => {
    if (detail.isError) onClose()
  }, [detail.isError, onClose])
  return (
    <PaymentFormModal
      open={!!detail.data}
      onClose={onClose}
      memberId={member.id}
      memberName={member.name}
      subscriptions={detail.data?.subscriptions ?? []}
      packages={detail.data?.packages ?? []}
      outstandingDues={detail.data?.outstanding_dues}
    />
  )
}
