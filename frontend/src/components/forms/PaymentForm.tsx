import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { PTPackage, Subscription } from '@/api/types'
import { useCreatePayment } from '@/api/hooks'
import { useTenant } from '@/lib/auth'
import { formatDate, formatMoney, fromDateTimeLocal, toDateTimeLocal } from '@/lib/format'
import { Button, Field, FormError, Input, Modal, Select, Textarea } from '@/components/ui'
import { applyApiErrors, numField } from './shared'

const schema = z.object({
  reference: z.string(), // "" | "sub:<id>" | "pkg:<id>"
  amount: numField({ min: 0.01 }),
  method: z.enum(['cash', 'card', 'upi', 'bank_transfer', 'other']),
  paid_at: z.string().min(1, 'Required'),
  notes: z.string(),
})
type Form = z.infer<typeof schema>

const hasDue = (x: { price: number; paid: number }) => x.price - x.paid > 0.004

/**
 * Pick the item a payment is most likely for when the caller did not say:
 * the current (or upcoming) membership with dues, else the most recent
 * membership with dues, else the oldest PT package with dues.
 */
function suggestReference(subs: Subscription[], pkgs: PTPackage[]): string {
  const today = new Date().toISOString().slice(0, 10)
  const owing = subs.filter((s) => s.status !== 'cancelled' && hasDue(s))
  const current = owing.filter((s) => s.end_date >= today).sort((a, b) => a.start_date.localeCompare(b.start_date))[0]
  if (current) return `sub:${current.id}`
  const latest = [...owing].sort((a, b) => b.end_date.localeCompare(a.end_date))[0]
  if (latest) return `sub:${latest.id}`
  const pkg = pkgs
    .filter((p) => p.status !== 'cancelled' && hasDue(p))
    .sort((a, b) => a.purchased_at.localeCompare(b.purchased_at))[0]
  if (pkg) return `pkg:${pkg.id}`
  return ''
}

export function PaymentFormModal({
  open,
  onClose,
  memberId,
  memberName,
  subscriptions = [],
  packages = [],
  outstandingDues,
  defaultReference,
}: {
  open: boolean
  onClose: () => void
  memberId: string
  memberName: string
  subscriptions?: Subscription[]
  packages?: PTPackage[]
  /** Member's total outstanding dues; suggested as the amount for a general payment. */
  outstandingDues?: number
  /** Pre-select a reference: { subscription_id } or { pt_package_id }. Omit to auto-pick the item with dues. */
  defaultReference?: { subscription_id?: string; pt_package_id?: string }
}) {
  const tenant = useTenant()
  const create = useCreatePayment()
  const [error, setError] = useState('')
  const form = useForm<Form>({
    resolver: zodResolver(schema),
    defaultValues: { reference: '', amount: '', method: 'upi', paid_at: toDateTimeLocal(new Date()), notes: '' },
  })
  const { register, handleSubmit, reset, watch, setValue, setError: setFieldError, formState } = form

  const unpaidSubs = subscriptions.filter((s) => s.status !== 'cancelled')
  const unpaidPkgs = packages.filter((p) => p.status !== 'cancelled')

  useEffect(() => {
    if (!open) return
    const ref = defaultReference?.subscription_id
      ? `sub:${defaultReference.subscription_id}`
      : defaultReference?.pt_package_id
        ? `pkg:${defaultReference.pt_package_id}`
        : suggestReference(subscriptions, packages)
    reset({ reference: ref, amount: '', method: 'upi', paid_at: toDateTimeLocal(new Date()), notes: '' })
    setError('')
    // Lists are only read when the modal opens; re-running on every refetch would clobber edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, reset, defaultReference])

  const reference = watch('reference')
  useEffect(() => {
    // Suggest the outstanding balance for the chosen reference.
    if (reference.startsWith('sub:')) {
      const s = unpaidSubs.find((x) => x.id === reference.slice(4))
      if (s) setValue('amount', String(Math.max(0, s.price - s.paid)))
    } else if (reference.startsWith('pkg:')) {
      const p = unpaidPkgs.find((x) => x.id === reference.slice(4))
      if (p) setValue('amount', String(Math.max(0, p.price - p.paid)))
    } else if (outstandingDues !== undefined && outstandingDues > 0) {
      setValue('amount', String(outstandingDues))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reference])

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    try {
      await create.mutateAsync({
        member_id: memberId,
        subscription_id: v.reference.startsWith('sub:') ? v.reference.slice(4) : null,
        pt_package_id: v.reference.startsWith('pkg:') ? v.reference.slice(4) : null,
        amount: Number(v.amount),
        method: v.method,
        paid_at: fromDateTimeLocal(v.paid_at),
        notes: v.notes,
      })
      onClose()
    } catch (e) {
      setError(applyApiErrors(e, setFieldError))
    }
  })

  const err = formState.errors
  const money = (n: number) => formatMoney(n, tenant.currency)

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Record payment"
      description={`Payment received from ${memberName}.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="payment-form" loading={formState.isSubmitting}>
            Record payment
          </Button>
        </>
      }
    >
      <form id="payment-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <Field
          label="For"
          hint="Pick the membership or PT package this pays for. A general payment is applied to the member's oldest outstanding balance."
        >
          <Select {...register('reference')}>
            <option value="">General (oldest balance first)</option>
            {unpaidSubs.length > 0 && (
              <optgroup label="Memberships">
                {unpaidSubs.map((s) => (
                  <option key={s.id} value={`sub:${s.id}`}>
                    {s.plan_name} ({formatDate(s.start_date)} – {formatDate(s.end_date)})
                    {s.price - s.paid > 0 ? ` · due ${money(s.price - s.paid)}` : ' · paid'}
                  </option>
                ))}
              </optgroup>
            )}
            {unpaidPkgs.length > 0 && (
              <optgroup label="PT packages">
                {unpaidPkgs.map((p) => (
                  <option key={p.id} value={`pkg:${p.id}`}>
                    {p.name} with {p.trainer_name}
                    {p.price - p.paid > 0 ? ` · due ${money(p.price - p.paid)}` : ' · paid'}
                  </option>
                ))}
              </optgroup>
            )}
          </Select>
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label={`Amount (${tenant.currency})`} error={err.amount?.message}>
            <Input type="number" min={0} step="0.01" autoFocus invalid={!!err.amount} {...register('amount')} />
          </Field>
          <Field label="Method" error={err.method?.message}>
            <Select {...register('method')}>
              <option value="upi">UPI</option>
              <option value="cash">Cash</option>
              <option value="card">Card</option>
              <option value="bank_transfer">Bank transfer</option>
              <option value="other">Other</option>
            </Select>
          </Field>
        </div>
        <Field label="Paid at" error={err.paid_at?.message}>
          <Input type="datetime-local" invalid={!!err.paid_at} {...register('paid_at')} />
        </Field>
        <Field label="Notes">
          <Textarea rows={2} placeholder="Receipt no., remarks…" {...register('notes')} />
        </Field>
      </form>
    </Modal>
  )
}
