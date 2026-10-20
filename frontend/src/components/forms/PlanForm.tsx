import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { MembershipPlan } from '@/api/types'
import { useCreatePlan, useUpdatePlan } from '@/api/hooks'
import { useTenant } from '@/lib/auth'
import { Button, Checkbox, Field, FormError, Input, Modal, Textarea } from '@/components/ui'
import { applyApiErrors, numField } from './shared'

const schema = z.object({
  name: z.string().trim().min(2, 'Name is required'),
  duration_days: numField({ min: 1, integer: true }),
  price: numField({ min: 0 }),
  description: z.string(),
  active: z.boolean(),
})
type Form = z.infer<typeof schema>

function toForm(p?: MembershipPlan): Form {
  return {
    name: p?.name ?? '',
    duration_days: p ? String(p.duration_days) : '30',
    price: p ? String(p.price) : '',
    description: p?.description ?? '',
    active: p?.active ?? true,
  }
}

const presets = [
  { label: '1 month', days: 30 },
  { label: '3 months', days: 90 },
  { label: '6 months', days: 180 },
  { label: '1 year', days: 365 },
]

export function PlanFormModal({ open, onClose, plan }: { open: boolean; onClose: () => void; plan?: MembershipPlan }) {
  const tenant = useTenant()
  const create = useCreatePlan()
  const update = useUpdatePlan()
  const [error, setError] = useState('')
  const form = useForm<Form>({ resolver: zodResolver(schema), defaultValues: toForm(plan) })
  const { register, handleSubmit, reset, setValue, watch, setError: setFieldError, formState } = form

  useEffect(() => {
    if (open) {
      reset(toForm(plan))
      setError('')
    }
  }, [open, plan, reset])

  const days = watch('duration_days')

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    const body = {
      name: v.name,
      duration_days: Number(v.duration_days),
      price: Number(v.price),
      description: v.description,
      active: v.active,
    }
    try {
      if (plan) await update.mutateAsync({ id: plan.id, body })
      else await create.mutateAsync(body)
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
      title={plan ? 'Edit plan' : 'New membership plan'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="plan-form" loading={formState.isSubmitting}>
            {plan ? 'Save changes' : 'Create plan'}
          </Button>
        </>
      }
    >
      <form id="plan-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <Field label="Plan name" error={err.name?.message}>
          <Input autoFocus placeholder="Monthly" invalid={!!err.name} {...register('name')} />
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Duration (days)" error={err.duration_days?.message}>
            <Input type="number" min={1} step={1} invalid={!!err.duration_days} {...register('duration_days')} />
            <div className="mt-1.5 flex flex-wrap gap-1">
              {presets.map((p) => (
                <button
                  key={p.days}
                  type="button"
                  onClick={() => setValue('duration_days', String(p.days), { shouldValidate: true })}
                  className={`chip ${Number(days) === p.days ? 'chip-on' : 'chip-off'}`}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </Field>
          <Field label={`Price (${tenant.currency})`} error={err.price?.message}>
            <Input type="number" min={0} step="0.01" invalid={!!err.price} {...register('price')} />
          </Field>
        </div>
        <Field label="Description">
          <Textarea rows={2} placeholder="What's included…" {...register('description')} />
        </Field>
        <Checkbox label="Active (available when adding memberships)" {...register('active')} />
      </form>
    </Modal>
  )
}
