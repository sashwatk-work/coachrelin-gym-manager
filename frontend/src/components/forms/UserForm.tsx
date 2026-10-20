import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { Role, Trainer, User } from '@/api/types'
import { useCreateUser, useTrainers, useUpdateUser } from '@/api/hooks'
import { Button, Field, FormError, Input, Modal, Select } from '@/components/ui'
import { applyApiErrors } from './shared'

const ROLE_OPTIONS: { value: Role; label: string; hint: string }[] = [
  { value: 'owner', label: 'Owner', hint: 'Full access — gym, team, plans' },
  { value: 'receptionist', label: 'Receptionist', hint: 'Front desk — members and payments' },
  { value: 'trainer', label: 'Trainer', hint: 'Own clients and sessions' },
]

const schema = z
  .object({
    full_name: z.string().trim().min(2, 'Name is required'),
    email: z.string().trim().email('Enter a valid email'),
    password: z.string(),
    roles: z.array(z.enum(['owner', 'receptionist', 'trainer'])).min(1, 'Pick at least one role'),
    trainer_id: z.string(),
  })
  .superRefine((v, ctx) => {
    if (v.roles.includes('trainer') && !v.trainer_id) {
      ctx.addIssue({ code: 'custom', path: ['trainer_id'], message: 'Pick the trainer this login belongs to' })
    }
  })
type Form = z.infer<typeof schema>

function toForm(u?: User, trainer?: Trainer): Form {
  return {
    full_name: u?.full_name ?? trainer?.full_name ?? '',
    email: u?.email ?? trainer?.email ?? '',
    password: '',
    roles: u?.roles?.length ? u.roles : trainer ? ['trainer'] : ['receptionist'],
    trainer_id: u?.trainer_id ?? trainer?.id ?? '',
  }
}

export function loginForTrainer(users: User[] | undefined, trainerId: string): User | undefined {
  return users?.find((u) => u.trainer_id === trainerId)
}

export function UserFormModal({
  open,
  onClose,
  user,
  trainer,
}: {
  open: boolean
  onClose: () => void
  user?: User
  trainer?: Trainer
}) {
  const forTrainer = !user && trainer ? trainer : undefined
  const create = useCreateUser()
  const update = useUpdateUser()
  const trainers = useTrainers({ status: 'active', page_size: 200 })
  const [error, setError] = useState('')
  const form = useForm<Form>({ resolver: zodResolver(schema), defaultValues: toForm(user, trainer) })
  const { register, handleSubmit, reset, watch, setValue, setError: setFieldError, formState } = form
  const roles = watch('roles')
  const hasTrainer = roles.includes('trainer')

  useEffect(() => {
    if (open) {
      reset(toForm(user, trainer))
      setError('')
    }
  }, [open, user, trainer, reset])

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    if (!user && v.password.length < 8) {
      setFieldError('password', { message: 'must be at least 8 characters' })
      return
    }
    if (user && v.password && v.password.length < 8) {
      setFieldError('password', { message: 'must be at least 8 characters' })
      return
    }
    const body = {
      full_name: v.full_name,
      email: v.email,
      roles: v.roles,
      trainer_id: v.roles.includes('trainer') ? v.trainer_id : null,
      ...(v.password ? { password: v.password } : {}),
    }
    try {
      if (user) await update.mutateAsync({ id: user.id, body })
      else await create.mutateAsync({ ...body, email: v.email, password: v.password })
      onClose()
    } catch (e) {
      setError(applyApiErrors(e, setFieldError))
    }
  })

  function toggleRole(role: Role) {
    const next = roles.includes(role) ? roles.filter((r) => r !== role) : [...roles, role]
    setValue('roles', next, { shouldValidate: true })
  }

  const err = formState.errors
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={user ? 'Edit login' : forTrainer ? `Login for ${forTrainer.full_name}` : 'Add login'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="user-form" loading={formState.isSubmitting}>
            {user ? 'Save changes' : forTrainer ? 'Create login' : 'Add login'}
          </Button>
        </>
      }
    >
      <form id="user-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <Field label="Full name" error={err.full_name?.message}>
          <Input autoFocus invalid={!!err.full_name} {...register('full_name')} />
        </Field>
        <Field label="Email" error={err.email?.message} hint={user ? 'Email cannot be changed' : undefined}>
          <Input type="email" disabled={!!user} invalid={!!err.email} {...register('email')} />
        </Field>
        {forTrainer ? (
          <>
            <input type="hidden" {...register('trainer_id')} />
            <p className="text-sm text-slate-500">
              This login is a trainer account for {forTrainer.full_name}. They will only see their own clients and sessions.
            </p>
          </>
        ) : (
          <>
            <Field label="Roles" error={err.roles?.message} hint="A login can hold more than one role. Permissions add together.">
              <div className="space-y-2">
                {ROLE_OPTIONS.map((opt) => {
                  const checked = roles.includes(opt.value)
                  return (
                    <label
                      key={opt.value}
                      className="flex cursor-pointer items-start gap-3 border border-ink/15 bg-paper px-3 py-2.5"
                    >
                      <input
                        type="checkbox"
                        className="mt-1 h-4 w-4 accent-ink"
                        checked={checked}
                        onChange={() => toggleRole(opt.value)}
                      />
                      <span>
                        <span className="block text-sm font-semibold text-ink">{opt.label}</span>
                        <span className="block text-xs text-slate-500">{opt.hint}</span>
                      </span>
                    </label>
                  )
                })}
              </div>
            </Field>
            {hasTrainer && (
              <Field label="Trainer profile" error={err.trainer_id?.message} hint="Links this login to a trainer on the roster.">
                <Select invalid={!!err.trainer_id} {...register('trainer_id')}>
                  <option value="">Choose a trainer…</option>
                  {trainers.data?.items.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.full_name}
                    </option>
                  ))}
                </Select>
              </Field>
            )}
          </>
        )}
        <Field label="Password" error={err.password?.message} hint={user ? 'Leave blank to keep the current password' : 'At least 8 characters'}>
          <Input type="password" autoComplete="new-password" invalid={!!err.password} {...register('password')} />
        </Field>
      </form>
    </Modal>
  )
}
