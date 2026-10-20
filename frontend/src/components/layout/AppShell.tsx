import { useState, type ReactNode } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '@/lib/auth'
import { can, consoleLabel } from '@/lib/rbac'
import { cn, initials } from '@/lib/format'
import type { Role } from '@/api/types'

type Icon = (props: { className?: string }) => ReactNode

const IconHome: Icon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M3 11.5 12 4l9 7.5" />
    <path d="M5 10v10h14V10" />
  </svg>
)
const IconUsers: Icon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="9" cy="8" r="3.5" />
    <path d="M2.5 20a6.5 6.5 0 0 1 13 0" />
    <circle cx="17" cy="9" r="2.5" />
    <path d="M15.5 14.5a5 5 0 0 1 6 5" />
  </svg>
)
const IconDumbbell: Icon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M4 10v4M7 8v8M17 8v8M20 10v4M7 12h10" />
  </svg>
)
const IconWhistle: Icon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="7" r="3.5" />
    <path d="M5 21a7 7 0 0 1 14 0" />
    <path d="M16 3.5 18 2M8 3.5 6 2" />
  </svg>
)
const IconCard: Icon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="6" width="18" height="12" rx="2" />
    <path d="M3 10h18M7 15h3" />
  </svg>
)
const IconCash: Icon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2.5" y="7" width="19" height="11" rx="2" />
    <circle cx="12" cy="12.5" r="2.5" />
    <path d="M6 10.5h.01M18 14.5h.01" />
  </svg>
)
const IconCog: Icon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="3" />
    <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
  </svg>
)
const IconMenu: Icon = ({ className }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
    <path d="M4 7h16M4 12h16M4 17h16" />
  </svg>
)

const nav: Array<{ to: string; label: string; icon: Icon; end?: boolean; show?: (roles: Role[] | undefined) => boolean }> = [
  { to: '/', label: 'Dashboard', icon: IconHome, end: true },
  { to: '/members', label: 'Members', icon: IconUsers },
  { to: '/training', label: 'Personal training', icon: IconDumbbell },
  { to: '/trainers', label: 'Trainers', icon: IconWhistle, show: (roles) => can(roles, 'viewTrainers') },
  { to: '/plans', label: 'Plans', icon: IconCard, show: (roles) => can(roles, 'viewPlans') },
  { to: '/payments', label: 'Payments', icon: IconCash, show: (roles) => can(roles, 'viewPayments') },
  { to: '/settings', label: 'Settings', icon: IconCog },
]

export function AppShell() {
  const { user, tenant, logout } = useAuth()
  const navigate = useNavigate()
  const [mobileOpen, setMobileOpen] = useState(false)

  async function handleLogout() {
    await logout()
    navigate('/login', { replace: true })
  }

  const sidebar = (
    <div className="flex h-full flex-col bg-ink text-paper">
      <div className="h-[3px] bg-signal" aria-hidden />
      <div className="flex items-center gap-3 px-5 py-5">
        <img
          src="/images/coach-relin-fitness-logo.webp"
          alt=""
          width={40}
          height={40}
          className="h-10 w-10 shrink-0 object-contain"
        />
        <div className="min-w-0">
          <p className="truncate font-display text-lg font-extrabold uppercase leading-none tracking-wide">
            {tenant?.name ?? 'Coach Relin Fitness'}
          </p>
          <p className="mt-1 text-[10px] font-extrabold uppercase tracking-label text-white/50">{consoleLabel(user?.roles)}</p>
        </div>
      </div>

      <nav className="mt-2 flex-1 space-y-0.5 px-3">
        {nav
          .filter((n) => !n.show || n.show(user?.roles))
          .map((n) => (
          <NavLink
            key={n.to}
            to={n.to}
            end={n.end}
            onClick={() => setMobileOpen(false)}
            className={({ isActive }) => cn('nav-link', isActive && 'nav-link-active')}
          >
            <n.icon className="h-[18px] w-[18px] shrink-0" />
            {n.label}
          </NavLink>
        ))}
      </nav>

      <div className="border-t border-white/10 px-4 py-4">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-signal font-display text-xs font-extrabold text-ink">
            {initials(user?.full_name ?? '?')}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold">{user?.full_name}</p>
            <p className="truncate text-xs text-white/50">{user?.email}</p>
          </div>
          <button
            type="button"
            onClick={handleLogout}
            className="p-1.5 text-white/50 hover:bg-signal hover:text-ink"
            title="Sign out"
            aria-label="Sign out"
          >
            <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M10 17l5-5-5-5M15 12H3M21 4v16" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  )

  return (
    <div className="min-h-screen lg:flex">
      <aside className="hidden w-64 shrink-0 lg:sticky lg:top-0 lg:block lg:h-screen">{sidebar}</aside>

      {mobileOpen && (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div className="absolute inset-0 bg-ink/50" onClick={() => setMobileOpen(false)} />
          <aside className="absolute inset-y-0 left-0 w-72 shadow-xl">{sidebar}</aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex items-center gap-3 border-b border-line bg-paper/90 px-4 py-3 backdrop-blur lg:hidden">
          <button
            type="button"
            className="p-1.5 text-ink hover:bg-signal"
            onClick={() => setMobileOpen(true)}
            aria-label="Open menu"
          >
            <IconMenu className="h-5 w-5" />
          </button>
          <span className="font-display text-lg font-extrabold uppercase tracking-wide">{tenant?.name}</span>
        </header>
        <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
