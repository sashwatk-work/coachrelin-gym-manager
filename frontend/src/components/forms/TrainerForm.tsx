import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { Trainer } from '@/api/types'
import { useCreateTrainer, useUpdateTrainer } from '@/api/hooks'
import { Button, Checkbox, Field, FormError, Input, Modal } from '@/components/ui'
import { applyApiErrors, nullIfEmpty, optionalEmail, optionalStr } from './shared'

const schema = z.object({
  full_name: z.string().trim().min(2, 'Name is required'),
  email: optionalEmail,
  phone: optionalStr,
  specialties: z.string(),
  active: z.boolean(),
})
type Form = z.infer<typeof schema>

function toForm(t?: Trainer): Form {
  return {
    full_name: t?.full_name ?? '',
    email: t?.email ?? '',
    phone: t?.phone ?? '',
    specialties: t?.specialties.join(', ') ?? '',
    active: t?.active ?? true,
  }
}

export function TrainerFormModal({ open, onClose, trainer }: { open: boolean; onClose: () => void; trainer?: Trainer }) {
  const create = useCreateTrainer()
  const update = useUpdateTrainer(trainer?.id ?? '')
  const [error, setError] = useState('')
  const form = useForm<Form>({ resolver: zodResolver(schema), defaultValues: toForm(trainer) })
  const { register, handleSubmit, reset, setError: setFieldError, formState } = form

  useEffect(() => {
    if (open) {
      reset(toForm(trainer))
      setError('')
    }
  }, [open, trainer, reset])

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    const body = {
      full_name: v.full_name,
      email: nullIfEmpty(v.email),
      phone: nullIfEmpty(v.phone),
      specialties: v.specialties
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
      active: v.active,
    }
    try {
      if (trainer) await update.mutateAsync(body)
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
      title={trainer ? 'Edit trainer' : 'Add trainer'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="trainer-form" loading={formState.isSubmitting}>
            {trainer ? 'Save changes' : 'Add trainer'}
          </Button>
        </>
      }
    >
      <form id="trainer-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <Field label="Full name" error={err.full_name?.message}>
          <Input autoFocus invalid={!!err.full_name} {...register('full_name')} />
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Phone" error={err.phone?.message}>
            <Input type="tel" {...register('phone')} />
          </Field>
          <Field label="Email" error={err.email?.message}>
            <Input type="email" invalid={!!err.email} {...register('email')} />
          </Field>
        </div>
        <Field label="Specialties" hint="Comma separated, e.g. Strength, HIIT, Mobility">
          <Input {...register('specialties')} />
        </Field>
        <Checkbox label="Active (available for new packages and sessions)" {...register('active')} />
      </form>
    </Modal>
  )
}
