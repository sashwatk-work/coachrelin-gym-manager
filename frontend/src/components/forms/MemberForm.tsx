import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { Member, MemberInput } from '@/api/types'
import { useCreateMember, useUpdateMember } from '@/api/hooks'
import { todayISO } from '@/lib/format'
import { Button, Field, FormError, Input, Modal, Select, Textarea } from '@/components/ui'
import { applyApiErrors, dateStr, nullIfEmpty, optionalDateStr, optionalEmail, optionalStr } from './shared'

const schema = z.object({
  full_name: z.string().trim().min(2, 'Name is required'),
  email: optionalEmail,
  phone: optionalStr,
  date_of_birth: optionalDateStr,
  gender: z.string(),
  joined_at: dateStr,
  status: z.enum(['active', 'inactive', 'frozen']),
  emergency_contact: optionalStr,
  notes: z.string(),
})
type Form = z.infer<typeof schema>

function toForm(m?: Member): Form {
  return {
    full_name: m?.full_name ?? '',
    email: m?.email ?? '',
    phone: m?.phone ?? '',
    date_of_birth: m?.date_of_birth ?? '',
    gender: m?.gender ?? '',
    joined_at: m?.joined_at ?? todayISO(),
    status: m?.status ?? 'active',
    emergency_contact: m?.emergency_contact ?? '',
    notes: m?.notes ?? '',
  }
}

function toInput(v: Form): MemberInput {
  return {
    full_name: v.full_name,
    email: nullIfEmpty(v.email),
    phone: nullIfEmpty(v.phone),
    date_of_birth: nullIfEmpty(v.date_of_birth),
    gender: nullIfEmpty(v.gender),
    joined_at: v.joined_at,
    status: v.status,
    emergency_contact: nullIfEmpty(v.emergency_contact),
    notes: v.notes,
  }
}

export function MemberFormModal({
  open,
  onClose,
  member,
  onSaved,
}: {
  open: boolean
  onClose: () => void
  member?: Member
  onSaved?: (m: Member) => void
}) {
  const create = useCreateMember()
  const update = useUpdateMember(member?.id ?? '')
  const [error, setError] = useState('')
  const form = useForm<Form>({ resolver: zodResolver(schema), defaultValues: toForm(member) })
  const { register, handleSubmit, reset, setError: setFieldError, formState } = form

  useEffect(() => {
    if (open) {
      reset(toForm(member))
      setError('')
    }
  }, [open, member, reset])

  const onSubmit = handleSubmit(async (values) => {
    setError('')
    try {
      const saved = member ? await update.mutateAsync(toInput(values)) : await create.mutateAsync(toInput(values))
      onSaved?.(saved)
      onClose()
    } catch (e) {
      setError(applyApiErrors(e, setFieldError))
    }
  })

  const busy = formState.isSubmitting
  const err = formState.errors

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={member ? 'Edit member' : 'Add member'}
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button type="submit" form="member-form" loading={busy}>
            {member ? 'Save changes' : 'Add member'}
          </Button>
        </>
      }
    >
      <form id="member-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Full name" error={err.full_name?.message} className="sm:col-span-2">
            <Input autoFocus invalid={!!err.full_name} {...register('full_name')} />
          </Field>
          <Field label="Phone" error={err.phone?.message}>
            <Input type="tel" placeholder="+91 98765 43210" {...register('phone')} />
          </Field>
          <Field label="Email" error={err.email?.message}>
            <Input type="email" invalid={!!err.email} {...register('email')} />
          </Field>
          <Field label="Date of birth" error={err.date_of_birth?.message}>
            <Input type="date" {...register('date_of_birth')} />
          </Field>
          <Field label="Gender">
            <Select {...register('gender')}>
              <option value="">Not specified</option>
              <option value="male">Male</option>
              <option value="female">Female</option>
              <option value="other">Other</option>
            </Select>
          </Field>
          <Field label="Joined on" error={err.joined_at?.message}>
            <Input type="date" invalid={!!err.joined_at} {...register('joined_at')} />
          </Field>
          <Field label="Status">
            <Select {...register('status')}>
              <option value="active">Active</option>
              <option value="frozen">Frozen</option>
              <option value="inactive">Inactive</option>
            </Select>
          </Field>
          <Field label="Emergency contact" className="sm:col-span-2">
            <Input placeholder="Name and phone" {...register('emergency_contact')} />
          </Field>
          <Field label="Notes" className="sm:col-span-2">
            <Textarea placeholder="Goals, injuries, preferences…" {...register('notes')} />
          </Field>
        </div>
      </form>
    </Modal>
  )
}
