import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { PTPackage } from '@/api/types'
import { useCreatePackage, useMembers, usePTPlans, useTrainers, useUpdatePackage } from '@/api/hooks'
import { useTenant } from '@/lib/auth'
import { addDays, formatMoney, todayISO } from '@/lib/format'
import { Button, Checkbox, Field, FormError, Input, Modal, Select } from '@/components/ui'
import { applyApiErrors, dateStr, numField, optionalDateStr } from './shared'

const CUSTOM = '__custom__'

const schema = z
  .object({
    member_id: z.string().min(1, 'Choose a member'),
    trainer_id: z.string().min(1, 'Choose a trainer'),
    pt_plan_id: z.string().min(1, 'Choose a package'),
    customise: z.boolean(),
    name: z.string(),
    total_sessions: numField({ min: 1, integer: true, required: false }),
    price: numField({ min: 0, required: false }),
    purchased_at: dateStr,
    expires_at: optionalDateStr,
    status: z.enum(['active', 'completed', 'expired', 'cancelled']),
  })
  .superRefine((v, ctx) => {
    const custom = v.pt_plan_id === CUSTOM || v.customise
    if (!custom) return
    if (v.name.trim().length < 2) ctx.addIssue({ code: 'custom', path: ['name'], message: 'Name is required' })
    if (v.total_sessions === '') ctx.addIssue({ code: 'custom', path: ['total_sessions'], message: 'Required' })
    if (v.price === '') ctx.addIssue({ code: 'custom', path: ['price'], message: 'Required' })
  })
type Form = z.infer<typeof schema>

function toForm(p?: PTPackage, memberId?: string): Form {
  const today = todayISO()
  return {
    member_id: p?.member_id ?? memberId ?? '',
    trainer_id: p?.trainer_id ?? '',
    pt_plan_id: p ? (p.pt_plan_id ?? CUSTOM) : '',
    customise: !!p,
    name: p?.name ?? '',
    total_sessions: p ? String(p.total_sessions) : '',
    price: p ? String(p.price) : '',
    purchased_at: p?.purchased_at ?? today,
    expires_at: p?.expires_at ?? '',
    status: p?.status ?? 'active',
  }
}

export function PackageFormModal({
  open,
  onClose,
  pkg,
  memberId,
  memberName,
}: {
  open: boolean
  onClose: () => void
  pkg?: PTPackage
  memberId?: string
  memberName?: string
}) {
  const tenant = useTenant()
  const trainers = useTrainers({ status: 'active', page_size: 200 })
  const plans = usePTPlans(true)
  const lockMember = !!memberId || !!pkg
  const members = useMembers({ page_size: 200 })
  const create = useCreatePackage()
  const update = useUpdatePackage()
  const [error, setError] = useState('')
  const form = useForm<Form>({ resolver: zodResolver(schema), defaultValues: toForm(pkg, memberId) })
  const { register, handleSubmit, reset, watch, setValue, setError: setFieldError, formState } = form

  useEffect(() => {
    if (!open) return
    reset(toForm(pkg, memberId))
    setError('')
  }, [open, pkg, memberId, reset])

  const trainerId = watch('trainer_id')
  useEffect(() => {
    if (open && !trainerId && trainers.data?.items[0]) setValue('trainer_id', trainers.data.items[0].id)
  }, [open, trainerId, trainers.data, setValue])

  const planId = watch('pt_plan_id')
  useEffect(() => {
    if (open && !pkg && !planId && plans.data) {
      setValue('pt_plan_id', plans.data[0]?.id ?? CUSTOM)
    }
  }, [open, pkg, planId, plans.data, setValue])

  const customise = watch('customise')
  const purchased = watch('purchased_at')
  const plan = useMemo(() => plans.data?.find((p) => p.id === planId), [plans.data, planId])
  const isCustom = planId === CUSTOM

  useEffect(() => {
    if (pkg || !plan) return
    setValue('name', plan.name)
    setValue('total_sessions', String(plan.sessions))
    setValue('price', String(plan.price))
    setValue('expires_at', plan.validity_days && /^\d{4}-\d{2}-\d{2}$/.test(purchased) ? addDays(purchased, plan.validity_days) : '')
  }, [plan, purchased, pkg, setValue])

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    const fromPlan = v.pt_plan_id !== CUSTOM
    const customising = v.customise || v.pt_plan_id === CUSTOM
    const body: Parameters<typeof create.mutateAsync>[0] = {
      member_id: v.member_id,
      trainer_id: v.trainer_id,
      pt_plan_id: fromPlan ? v.pt_plan_id : null,
      purchased_at: v.purchased_at,
      status: v.status,
    }
    if (customising) {
      body.name = v.name.trim()
      body.total_sessions = Number(v.total_sessions)
      body.price = Number(v.price)
      // Empty string (not null) tells the API to skip the plan's validity.
      body.expires_at = v.expires_at
    }
    try {
      if (pkg) await update.mutateAsync({ id: pkg.id, body })
      else await create.mutateAsync(body)
      onClose()
    } catch (e) {
      setError(applyApiErrors(e, setFieldError))
    }
  })

  const err = formState.errors
  const money = (n: number) => formatMoney(n, tenant.currency)
  const showFields = customise || isCustom

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={pkg ? 'Edit PT package' : 'Buy PT package'}
      description={memberName ? `For ${memberName}` : 'Sell a block of personal-training sessions, same way as a membership.'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="package-form" loading={formState.isSubmitting}>
            {pkg ? 'Save changes' : 'Create package'}
          </Button>
        </>
      }
    >
      <form id="package-form" onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        {!lockMember && (
          <Field label="Member" error={err.member_id?.message}>
            <Select invalid={!!err.member_id} {...register('member_id')}>
              <option value="">Choose a member…</option>
              {members.data?.items.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.full_name}
                  {m.phone ? ` · ${m.phone}` : ''}
                </option>
              ))}
            </Select>
          </Field>
        )}
        <Field label="Package" error={err.pt_plan_id?.message} hint="Defined under Plans → Personal training. Pick Custom to enter sessions and price yourself.">
          <Select invalid={!!err.pt_plan_id} {...register('pt_plan_id')}>
            {plans.data?.length === 0 && <option value="">No packages yet — add one under Plans</option>}
            {plans.data?.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} · {p.sessions} sessions · {money(p.price)}
              </option>
            ))}
            <option value={CUSTOM}>Custom…</option>
          </Select>
        </Field>
        <Field label="Trainer" error={err.trainer_id?.message}>
          <Select invalid={!!err.trainer_id} {...register('trainer_id')}>
            {trainers.data?.items.length === 0 && <option value="">No active trainers — add one first</option>}
            {trainers.data?.items.map((t) => (
              <option key={t.id} value={t.id}>
                {t.full_name}
                {t.specialties.length ? ` · ${t.specialties.join(', ')}` : ''}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Purchased on" error={err.purchased_at?.message}>
          <Input type="date" invalid={!!err.purchased_at} {...register('purchased_at')} />
        </Field>

        {plan && !isCustom && !showFields && (
          <p className="rounded-lg bg-slate-50 px-3 py-2 text-sm text-slate-600">
            {plan.sessions} sessions · {money(plan.price)}
            {plan.validity_days ? ` · valid ${plan.validity_days} days` : ' · no expiry'}
            {plan.description ? <span className="mt-1 block text-slate-500">{plan.description}</span> : null}
          </p>
        )}

        {!isCustom && (
          <Checkbox label="Customise sessions, price or expiry for this member" {...register('customise')} />
        )}

        {showFields && (
          <>
            <Field label="Package name" error={err.name?.message}>
              <Input invalid={!!err.name} {...register('name')} />
            </Field>
            <div className="grid grid-cols-2 gap-4">
              <Field label="Sessions" error={err.total_sessions?.message}>
                <Input type="number" min={1} step={1} invalid={!!err.total_sessions} {...register('total_sessions')} />
              </Field>
              <Field label={`Price (${tenant.currency})`} error={err.price?.message}>
                <Input type="number" min={0} step="0.01" invalid={!!err.price} {...register('price')} />
              </Field>
            </div>
            <Field label="Expires on" error={err.expires_at?.message} hint="Optional">
              <Input type="date" {...register('expires_at')} />
            </Field>
          </>
        )}

        {pkg && (
          <Field label="Status">
            <Select {...register('status')}>
              <option value="active">Active</option>
              <option value="completed">Completed</option>
              <option value="expired">Expired</option>
              <option value="cancelled">Cancelled</option>
            </Select>
          </Field>
        )}
      </form>
    </Modal>
  )
}
