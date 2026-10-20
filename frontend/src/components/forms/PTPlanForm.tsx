import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { PTPlan } from '@/api/types'
import { useCreatePTPlan, useUpdatePTPlan } from '@/api/hooks'
import { useTenant } from '@/lib/auth'
import { Button, Checkbox, Field, FormError, Input, Modal, Textarea } from '@/components/ui'
import { applyApiErrors, numField } from './shared'

const schema = z.object({
  name: z.string().trim().min(2, 'Name is required'),
  sessions: numField({ min: 1, integer: true }),
  validity_days: numField({ min: 1, integer: true, required: false }),
  price: numField({ min: 0 }),
  description: z.string(),
  active: z.boolean(),
})
type Form = z.infer<typeof schema>

function toForm(p?: PTPlan): Form {
  return {
    name: p?.name ?? '',
    sessions: p ? String(p.sessions) : '8',
    validity_days: p?.validity_days != null ? String(p.validity_days) : '90',
    price: p ? String(p.price) : '',
    description: p?.description ?? '',
    active: p?.active ?? true,
  }
}

const sessionPresets = [4, 8, 12, 20, 24]
const validityPresets = [
  { label: 'None', days: '' },
  { label: '1 month', days: '30' },
  { label: '3 months', days: '90' },
  { label: '4 months', days: '120' },
  { label: '6 months', days: '180' },
]

export function PTPlanFormModal({ open, onClose, plan }: { open: boolean; onClose: () => void; plan?: PTPlan }) {
  const tenant = useTenant()
  const create = useCreatePTPlan()
  const update = useUpdatePTPlan()
  const [error, setError] = useState('')
  const form = useForm<Form>({ resolver: zodResolver(schema), defaultValues: toForm(plan) })
  const { register, handleSubmit, reset, setValue, watch, setError: setFieldError, formState } = form

  useEffect(() => {
    if (open) {
      reset(toForm(plan))
      setError('')
    }
  }, [open, plan, reset])

  const sessions = watch('sessions')
  const validity = watch('validity_days')

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    const body = {
      name: v.name,
      sessions: Number(v.sessions),
      validity_days: v.validity_days === '' ? null : Number(v.validity_days),
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
      title={plan ? 'Edit PT package' : 'New PT package'}
      description="A reusable block of sessions you can sell to members. Price and sessions can still be customised per member."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="pt-plan-form" loading={formState.isSubmitting}>
            {plan ? 'Save changes' : 'Create package'}
          </Button>
        </>
      }
    >
      <form id="pt-plan-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <Field label="Package name" error={err.name?.message}>
          <Input autoFocus placeholder="8-session PT pack" invalid={!!err.name} {...register('name')} />
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Sessions" error={err.sessions?.message}>
            <Input type="number" min={1} step={1} invalid={!!err.sessions} {...register('sessions')} />
            <div className="mt-1.5 flex flex-wrap gap-1">
              {sessionPresets.map((n) => (
                <button
                  key={n}
                  type="button"
                  onClick={() => setValue('sessions', String(n), { shouldValidate: true })}
                  className={`chip ${Number(sessions) === n ? 'chip-on' : 'chip-off'}`}
                >
                  {n}
                </button>
              ))}
            </div>
          </Field>
          <Field label={`Price (${tenant.currency})`} error={err.price?.message}>
            <Input type="number" min={0} step="0.01" invalid={!!err.price} {...register('price')} />
          </Field>
        </div>
        <Field label="Valid for (days)" error={err.validity_days?.message} hint="Leave blank if the package never expires.">
          <Input type="number" min={1} step={1} invalid={!!err.validity_days} {...register('validity_days')} />
          <div className="mt-1.5 flex flex-wrap gap-1">
            {validityPresets.map((p) => (
              <button
                key={p.label}
                type="button"
                onClick={() => setValue('validity_days', p.days, { shouldValidate: true })}
                className={`chip ${(p.days === '' ? validity === '' : Number(validity) === Number(p.days)) ? 'chip-on' : 'chip-off'}`}
              >
                {p.label}
              </button>
            ))}
          </div>
        </Field>
        <Field label="Description">
          <Textarea rows={2} placeholder="What's included…" {...register('description')} />
        </Field>
        <Checkbox label="Active (available when selling a package to a member)" {...register('active')} />
      </form>
    </Modal>
  )
}
