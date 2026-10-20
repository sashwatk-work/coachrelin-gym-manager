import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useCreateSubscription, usePlans } from '@/api/hooks'
import { useTenant } from '@/lib/auth'
import { addDays, formatDate, formatMoney, todayISO } from '@/lib/format'
import { Button, Field, FormError, Input, Modal, Select } from '@/components/ui'
import { applyApiErrors, dateStr, numField } from './shared'

const schema = z.object({
  plan_id: z.string().min(1, 'Choose a plan'),
  start_date: dateStr,
  price: numField({ min: 0 }),
})
type Form = z.infer<typeof schema>

export function SubscriptionFormModal({
  open,
  onClose,
  memberId,
  memberName,
  suggestedStart,
}: {
  open: boolean
  onClose: () => void
  memberId: string
  memberName: string
  /** e.g. the day after the current membership ends */
  suggestedStart?: string
}) {
  const tenant = useTenant()
  const plans = usePlans(true)
  const create = useCreateSubscription(memberId)
  const [error, setError] = useState('')
  const form = useForm<Form>({
    resolver: zodResolver(schema),
    defaultValues: { plan_id: '', start_date: suggestedStart ?? todayISO(), price: '' },
  })
  const { register, handleSubmit, reset, watch, setValue, setError: setFieldError, formState } = form

  useEffect(() => {
    if (open) {
      reset({ plan_id: plans.data?.[0]?.id ?? '', start_date: suggestedStart ?? todayISO(), price: plans.data?.[0] ? String(plans.data[0].price) : '' })
      setError('')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, reset, suggestedStart, plans.data])

  const planId = watch('plan_id')
  const start = watch('start_date')
  const plan = useMemo(() => plans.data?.find((p) => p.id === planId), [plans.data, planId])

  // Keep price in sync with the chosen plan unless the user typed something.
  useEffect(() => {
    if (plan) setValue('price', String(plan.price))
  }, [plan, setValue])

  const end = plan && /^\d{4}-\d{2}-\d{2}$/.test(start) ? addDays(start, plan.duration_days - 1) : null

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    try {
      await create.mutateAsync({ plan_id: v.plan_id, start_date: v.start_date, price: Number(v.price) })
      onClose()
    } catch (e) {
      setError(applyApiErrors(e, setFieldError))
    }
  })

  const err = formState.errors
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Add membership"
      description={`Start or renew a membership for ${memberName}.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="subscription-form" loading={formState.isSubmitting}>
            Add membership
          </Button>
        </>
      }
    >
      <form id="subscription-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <Field label="Plan" error={err.plan_id?.message}>
          <Select invalid={!!err.plan_id} {...register('plan_id')}>
            {plans.data?.length === 0 && <option value="">No active plans — create one first</option>}
            {plans.data?.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} · {p.duration_days} days · {formatMoney(p.price, tenant.currency)}
              </option>
            ))}
          </Select>
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Start date" error={err.start_date?.message}>
            <Input type="date" invalid={!!err.start_date} {...register('start_date')} />
          </Field>
          <Field label="Price" error={err.price?.message} hint={plan ? `Plan price ${formatMoney(plan.price, tenant.currency)}` : undefined}>
            <Input type="number" min={0} step="0.01" invalid={!!err.price} {...register('price')} />
          </Field>
        </div>
        {end && (
          <p className="rounded-lg bg-slate-50 px-3 py-2 text-sm text-slate-600">
            Membership runs <span className="font-medium text-slate-900">{formatDate(start)}</span> to{' '}
            <span className="font-medium text-slate-900">{formatDate(end)}</span>.
          </p>
        )}
      </form>
    </Modal>
  )
}
