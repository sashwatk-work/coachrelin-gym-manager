import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { PTPackage, PTSession } from '@/api/types'
import { useCreateSession, usePackages, useTrainers, useUpdateSession } from '@/api/hooks'
import { fromDateTimeLocal, toDateTimeLocal } from '@/lib/format'
import { Button, Field, FormError, Input, Modal, Select, Textarea } from '@/components/ui'
import { applyApiErrors, numField } from './shared'

const schema = z.object({
  package_id: z.string().min(1, 'Choose a package'),
  trainer_id: z.string(),
  scheduled_at: z.string().min(1, 'Pick a date and time'),
  duration_minutes: numField({ min: 15, integer: true }),
  status: z.enum(['scheduled', 'completed', 'cancelled', 'no_show']),
  notes: z.string(),
})
type Form = z.infer<typeof schema>

function nextSlot(): string {
  const d = new Date()
  d.setMinutes(0, 0, 0)
  d.setHours(d.getHours() + 1)
  return toDateTimeLocal(d)
}

function toForm(s?: PTSession, pkg?: PTPackage): Form {
  return {
    package_id: s?.package_id ?? pkg?.id ?? '',
    trainer_id: s?.trainer_id ?? pkg?.trainer_id ?? '',
    scheduled_at: s ? toDateTimeLocal(new Date(s.scheduled_at)) : nextSlot(),
    duration_minutes: String(s?.duration_minutes ?? 60),
    status: s?.status ?? 'scheduled',
    notes: s?.notes ?? '',
  }
}

export function SessionFormModal({
  open,
  onClose,
  session,
  pkg,
  memberId,
}: {
  open: boolean
  onClose: () => void
  session?: PTSession
  /** Lock to a package (opened from package/member page). */
  pkg?: PTPackage
  /** Restrict the package list to a member. */
  memberId?: string
}) {
  const packages = usePackages({ status: 'active', member_id: memberId, page_size: 200 })
  const trainers = useTrainers({ page_size: 200 })
  const create = useCreateSession()
  const update = useUpdateSession()
  const [error, setError] = useState('')
  const form = useForm<Form>({ resolver: zodResolver(schema), defaultValues: toForm(session, pkg) })
  const { register, handleSubmit, reset, watch, setValue, setError: setFieldError, formState } = form

  useEffect(() => {
    if (open) {
      reset(toForm(session, pkg))
      setError('')
    }
  }, [open, session, pkg, reset])

  const packageId = watch('package_id')
  const selected = packages.data?.items.find((p) => p.id === packageId) ?? pkg
  // Default trainer to the package's trainer when the package changes.
  useEffect(() => {
    if (!session && selected) setValue('trainer_id', selected.trainer_id)
  }, [selected, session, setValue])

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    const body = {
      package_id: v.package_id,
      trainer_id: v.trainer_id || null,
      scheduled_at: fromDateTimeLocal(v.scheduled_at),
      duration_minutes: Number(v.duration_minutes),
      status: v.status,
      notes: v.notes,
    }
    try {
      if (session) await update.mutateAsync({ id: session.id, body })
      else await create.mutateAsync(body)
      onClose()
    } catch (e) {
      setError(applyApiErrors(e, setFieldError))
    }
  })

  const err = formState.errors
  const left = selected ? selected.sessions_remaining - selected.sessions_scheduled : null

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={session ? 'Edit session' : 'Schedule session'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="session-form" loading={formState.isSubmitting}>
            {session ? 'Save changes' : 'Schedule'}
          </Button>
        </>
      }
    >
      <form id="session-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <Field
          label="Package"
          error={err.package_id?.message}
          hint={
            selected
              ? `${selected.member_name} · ${selected.sessions_remaining} of ${selected.total_sessions} left${left !== null && left >= 0 ? `, ${left} unscheduled` : ''}`
              : undefined
          }
        >
          <Select invalid={!!err.package_id} disabled={!!session || !!pkg} {...register('package_id')}>
            <option value="">Choose a package…</option>
            {(pkg ? [pkg] : packages.data?.items ?? []).map((p) => (
              <option key={p.id} value={p.id}>
                {p.member_name} · {p.name} ({p.sessions_remaining} left)
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Trainer" hint="Defaults to the package's trainer; change for a substitute.">
          <Select {...register('trainer_id')}>
            {trainers.data?.items.map((t) => (
              <option key={t.id} value={t.id}>
                {t.full_name}
                {!t.active ? ' (inactive)' : ''}
              </option>
            ))}
          </Select>
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="Date & time" error={err.scheduled_at?.message}>
            <Input type="datetime-local" invalid={!!err.scheduled_at} {...register('scheduled_at')} />
          </Field>
          <Field label="Duration (min)" error={err.duration_minutes?.message}>
            <Input type="number" min={15} step={15} invalid={!!err.duration_minutes} {...register('duration_minutes')} />
          </Field>
        </div>
        <Field label="Status">
          <Select {...register('status')}>
            <option value="scheduled">Scheduled</option>
            <option value="completed">Completed</option>
            <option value="no_show">No-show</option>
            <option value="cancelled">Cancelled</option>
          </Select>
        </Field>
        <Field label="Notes">
          <Textarea rows={2} placeholder="Focus for this session…" {...register('notes')} />
        </Field>
      </form>
    </Modal>
  )
}
