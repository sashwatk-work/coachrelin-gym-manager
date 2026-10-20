import { useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useAuth } from '@/lib/auth'
import { errorMessage } from '@/lib/format'
import { Button, Field, FormError, Input } from '@/components/ui'

const schema = z.object({
  email: z.string().trim().email('Enter a valid email'),
  password: z.string().min(1, 'Password is required'),
})
type Form = z.infer<typeof schema>

export function AuthLayout({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen bg-paper">
      <div className="hidden w-1/2 flex-col justify-between bg-ink p-12 text-paper lg:flex">
        <div className="flex items-center gap-3">
          <img src="/images/coach-relin-fitness-logo.webp" alt="" width={48} height={48} className="h-12 w-12 object-contain" />
          <span className="font-display text-2xl font-extrabold uppercase tracking-wide">Coach Relin Fitness</span>
        </div>
        <div>
          <p className="max-w-lg font-display text-5xl font-semibold uppercase leading-[0.95] tracking-tight">
            Members, memberships, trainers and <em className="not-italic text-signal-ink" style={{ background: 'var(--signal)', paddingInline: '0.12em' }}>PT sessions</em> in one place.
          </p>
          <p className="mt-6 max-w-md text-sm text-white/55">
            The console for Coach Relin Fitness. Staff sign in with the login Relin creates for them.
          </p>
        </div>
        <p className="text-[10px] font-extrabold uppercase tracking-label text-white/40">Relin · owner</p>
      </div>
      <div className="flex w-full items-center justify-center px-6 py-12 lg:w-1/2">
        <div className="w-full max-w-sm">
          <img src="/images/coach-relin-fitness-logo.webp" alt="" width={48} height={48} className="mb-6 h-12 w-12 object-contain lg:hidden" />
          <h1 className="font-display text-4xl font-semibold uppercase tracking-tight">{title}</h1>
          <p className="mt-2 text-sm text-muted">{subtitle}</p>
          <div className="mt-8">{children}</div>
        </div>
      </div>
    </div>
  )
}

export default function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation() as { state?: { from?: string } }
  const [error, setError] = useState<string | null>(null)
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<Form>({ resolver: zodResolver(schema), defaultValues: { email: '', password: '' } })

  const onSubmit = handleSubmit(async (values) => {
    setError(null)
    try {
      await login(values.email, values.password)
      navigate(location.state?.from ?? '/', { replace: true })
    } catch (e) {
      setError(errorMessage(e))
    }
  })

  return (
    <AuthLayout title="Welcome back" subtitle="Sign in to Coach Relin Fitness.">
      <form onSubmit={onSubmit} className="space-y-4" noValidate>
        <FormError message={error} />
        <Field label="Email" error={errors.email?.message} htmlFor="email">
          <Input id="email" type="email" autoComplete="email" placeholder="owner@coachrelin.fit" invalid={!!errors.email} {...register('email')} />
        </Field>
        <Field label="Password" error={errors.password?.message} htmlFor="password">
          <Input id="password" type="password" autoComplete="current-password" invalid={!!errors.password} {...register('password')} />
        </Field>
        <Button type="submit" className="w-full" loading={isSubmitting}>
          Sign in
        </Button>
      </form>
    </AuthLayout>
  )
}
