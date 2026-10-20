import { createContext, useCallback, useContext, useEffect, useMemo, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Navigate, useLocation } from 'react-router-dom'
import { api, ApiError, onUnauthorized } from '@/api/client'
import { keys } from '@/api/hooks'
import type { AuthResponse, Role, Tenant, User } from '@/api/types'
import { hasRole, normalizeUser } from '@/lib/rbac'

interface AuthState {
  user: User | null
  tenant: Tenant | null
  loading: boolean
  login: (email: string, password: string) => Promise<AuthResponse>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient()

  const me = useQuery({
    queryKey: keys.me,
    queryFn: async () => {
      try {
        return await api.get<AuthResponse>('/auth/me')
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) return null
        throw e
      }
    },
    staleTime: 5 * 60_000,
    retry: false,
  })

  // When any request returns 401, drop the cached session.
  useEffect(() => {
    return onUnauthorized(() => {
      qc.setQueryData(keys.me, null)
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'me' })
    })
  }, [qc])

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await api.post<AuthResponse>('/auth/login', { email, password })
      qc.setQueryData(keys.me, res)
      return res
    },
    [qc],
  )

  const logout = useCallback(async () => {
    await api.post('/auth/logout')
    qc.setQueryData(keys.me, null)
    qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'me' })
  }, [qc])

  const value = useMemo<AuthState>(
    () => ({
      user: me.data?.user ? normalizeUser(me.data.user) : null,
      tenant: me.data?.tenant ?? null,
      loading: me.isLoading,
      login,
      logout,
    }),
    [me.data, me.isLoading, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}

/** Returns the tenant; only valid inside authenticated routes. */
export function useTenant(): Tenant {
  const { tenant } = useAuth()
  if (!tenant) throw new Error('useTenant used outside an authenticated route')
  return tenant
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  const location = useLocation()
  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-paper text-sm text-muted">Loading…</div>
    )
  }
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <>{children}</>
}

export function RedirectIfAuthed({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) return null
  if (user) return <Navigate to="/" replace />
  return <>{children}</>
}

export function RequireRole({ roles, children }: { roles: Role[]; children: ReactNode }) {
  const { user, loading } = useAuth()
  if (loading) return null
  if (!user) return <Navigate to="/login" replace />
  if (!roles.some((r) => hasRole(user.roles, r))) return <Navigate to="/" replace />
  return <>{children}</>
}
