import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import type { User } from '@/api/types'
import { useDeleteUser, useUpdateTenant, useUsers } from '@/api/hooks'
import { useAuth, useTenant } from '@/lib/auth'
import { can, hasRole, rolesLabel } from '@/lib/rbac'
import { errorMessage, formatDate, formatMoney } from '@/lib/format'
import { Button, ConfirmDialog, Field, FormError, Input, PageHeader } from '@/components/ui'
import { applyApiErrors } from '@/components/forms/shared'
import { UserFormModal } from '@/components/forms/UserForm'

const gstinRe = /^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$/

const schema = z.object({
  name: z.string().trim().min(2, 'Gym name is required'),
  timezone: z.string().trim().min(1, 'Required'),
  currency: z.string().trim().length(3, '3-letter ISO code, e.g. INR'),
  gstin: z
    .string()
    .transform((s) => s.replace(/\s/g, '').toUpperCase())
    .refine((s) => s === '' || gstinRe.test(s), '15-character GSTIN, e.g. 32ABCDE1234F1Z5'),
  gst_percent: z
    .string()
    .trim()
    .refine((s) => s === '' || (!Number.isNaN(Number(s)) && Number(s) >= 0 && Number(s) <= 100), 'Enter a rate from 0 to 100'),
})
type Form = z.infer<typeof schema>

export default function SettingsPage() {
  const tenant = useTenant()
  const { user } = useAuth()
  const update = useUpdateTenant()
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const form = useForm<Form>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: tenant.name,
      timezone: tenant.timezone,
      currency: tenant.currency,
      gstin: tenant.gstin ?? '',
      gst_percent: String(tenant.gst_percent ?? 0),
    },
  })
  const { register, handleSubmit, reset, watch, setError: setFieldError, formState } = form

  useEffect(() => {
    reset({
      name: tenant.name,
      timezone: tenant.timezone,
      currency: tenant.currency,
      gstin: tenant.gstin ?? '',
      gst_percent: String(tenant.gst_percent ?? 0),
    })
  }, [tenant, reset])

  const currency = watch('currency')
  const tzValid = (() => {
    try {
      new Intl.DateTimeFormat(undefined, { timeZone: watch('timezone') })
      return true
    } catch {
      return false
    }
  })()

  const onSubmit = handleSubmit(async (v) => {
    setError('')
    setSaved(false)
    try {
      await update.mutateAsync({
        name: v.name,
        timezone: v.timezone,
        currency: v.currency.toUpperCase(),
        gstin: v.gstin || null,
        gst_percent: v.gst_percent === '' ? 0 : Number(v.gst_percent),
      })
      setSaved(true)
    } catch (e) {
      setError(applyApiErrors(e, setFieldError))
    }
  })

  const err = formState.errors
  const isOwner = hasRole(user?.roles, 'owner')

  return (
    <div className="max-w-2xl">
      <PageHeader title="Settings" subtitle={isOwner ? 'This gym’s profile, team logins and regional preferences.' : 'Your account and this gym’s profile.'} />

      <form onSubmit={onSubmit} className="card space-y-5 p-6" noValidate>
        <FormError message={error} />
        {saved && <div className="border border-ink bg-signal px-3 py-2 text-sm font-semibold text-ink">Settings saved.</div>}
        <Field label="Gym name" error={err.name?.message}>
          <Input disabled={!isOwner} invalid={!!err.name} {...register('name')} />
        </Field>
        <div className="grid gap-5 sm:grid-cols-2">
          <Field label="Timezone" error={err.timezone?.message ?? (!tzValid ? 'Unknown timezone' : undefined)} hint="IANA name, e.g. Asia/Kolkata. Used for 'today' and expiry calculations.">
            <Input disabled={!isOwner} list="tz-list" invalid={!!err.timezone || !tzValid} {...register('timezone')} />
            <datalist id="tz-list">
              {['Asia/Kolkata', 'Asia/Dubai', 'Asia/Singapore', 'Europe/London', 'Europe/Berlin', 'America/New_York', 'America/Chicago', 'America/Los_Angeles', 'Australia/Sydney', 'UTC'].map((tz) => (
                <option key={tz} value={tz} />
              ))}
            </datalist>
          </Field>
          <Field label="Currency" error={err.currency?.message} hint={currency.length === 3 ? `Example: ${formatMoney(1500, currency.toUpperCase())}` : undefined}>
            <Input disabled={!isOwner} maxLength={3} className="uppercase" invalid={!!err.currency} {...register('currency')} />
          </Field>
        </div>
        <div className="grid gap-5 sm:grid-cols-2">
          <Field
            label="GSTIN"
            error={err.gstin?.message}
            hint="Printed on bills. Leave blank if this location is not GST-registered."
          >
            <Input
              disabled={!isOwner}
              maxLength={15}
              className="uppercase tracking-wide"
              placeholder="15-character GSTIN"
              invalid={!!err.gstin}
              {...register('gstin')}
            />
          </Field>
          <Field
            label="GST %"
            error={err.gst_percent?.message}
            hint="Shown on bills. Plan and package prices are treated as GST-inclusive."
          >
            <Input
              disabled={!isOwner}
              type="number"
              min={0}
              max={100}
              step="0.01"
              invalid={!!err.gst_percent}
              {...register('gst_percent')}
            />
          </Field>
        </div>
        <div className="flex items-center justify-between border-t border-slate-100 pt-5">
          <p className="text-xs text-slate-400">
            Location <span className="font-mono">{tenant.slug}</span> · created {formatDate(tenant.created_at.slice(0, 10))}
          </p>
          <Button type="submit" loading={formState.isSubmitting} disabled={!isOwner || !tzValid}>
            Save changes
          </Button>
        </div>
      </form>

      <div className="card mt-6 p-6">
        <h2 className="font-display text-xl font-semibold uppercase tracking-tight">Your account</h2>
        <dl className="mt-3 grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-[10px] font-extrabold uppercase tracking-label text-muted">Name</dt>
            <dd className="mt-0.5 text-slate-800">{user?.full_name}</dd>
          </div>
          <div>
            <dt className="text-[10px] font-extrabold uppercase tracking-label text-muted">Email</dt>
            <dd className="mt-0.5 text-slate-800">{user?.email}</dd>
          </div>
          <div>
            <dt className="text-[10px] font-extrabold uppercase tracking-label text-muted">Roles</dt>
            <dd className="mt-0.5 text-slate-800">{rolesLabel(user?.roles)}</dd>
          </div>
        </dl>
      </div>

      {can(user?.roles, 'manageUsers') && <TeamCard currentUserId={user?.id} />}

      <div className="card mt-6 border-slate-200 p-6">
        <h2 className="font-display text-xl font-semibold uppercase tracking-tight">This gym</h2>
        <p className="mt-2 text-sm text-slate-600">
          Members, payments and sessions belong to this location. When Relin adds more gyms later, each one will keep its own members and
          staff.
        </p>
      </div>
    </div>
  )
}

function TeamCard({ currentUserId }: { currentUserId?: string }) {
  const users = useUsers()
  const del = useDeleteUser()
  const [modal, setModal] = useState<{ open: boolean; user?: User }>({ open: false })
  const [removing, setRemoving] = useState<User | null>(null)
  const [deleteError, setDeleteError] = useState('')

  return (
    <div className="card mt-6 p-6">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="font-display text-xl font-semibold uppercase tracking-tight">Team</h2>
          <p className="mt-1 text-sm text-slate-500">Owner, receptionist and trainer logins. A person can hold more than one role.</p>
        </div>
        <Button size="sm" onClick={() => setModal({ open: true })}>
          Add login
        </Button>
      </div>
      <ul className="mt-4 divide-y divide-slate-100">
        {(users.data?.items ?? []).map((u) => (
          <li key={u.id} className="flex items-center gap-3 py-3 first:pt-0">
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-slate-800">
                {u.full_name}
                {u.id === currentUserId ? <span className="ml-2 text-xs font-normal text-slate-400">you</span> : null}
              </p>
              <p className="truncate text-xs text-slate-500">
                {u.email}
                {u.trainer_name ? ` · ${u.trainer_name}` : ''}
              </p>
            </div>
            <span className="text-xs font-medium text-slate-500">{rolesLabel(u.roles)}</span>
            <Button size="sm" variant="ghost" onClick={() => setModal({ open: true, user: u })}>
              Edit
            </Button>
            {u.id !== currentUserId && (
              <Button
                size="sm"
                variant="ghost"
                className="text-error hover:bg-[#f3c4bf]"
                onClick={() => {
                  setDeleteError('')
                  setRemoving(u)
                }}
              >
                Remove
              </Button>
            )}
          </li>
        ))}
      </ul>
      <UserFormModal open={modal.open} onClose={() => setModal({ open: false })} user={modal.user} />
      <ConfirmDialog
        open={removing !== null}
        onClose={() => setRemoving(null)}
        title="Remove login?"
        description={deleteError || `Remove ${removing?.full_name}'s access? They will no longer be able to sign in.`}
        loading={del.isPending}
        onConfirm={async () => {
          if (!removing) return
          try {
            await del.mutateAsync(removing.id)
            setRemoving(null)
          } catch (e) {
            setDeleteError(errorMessage(e))
          }
        }}
      />
    </div>
  )
}
