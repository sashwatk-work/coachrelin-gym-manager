import { useMutation, useQuery, useQueryClient, keepPreviousData } from '@tanstack/react-query'
import { api } from './client'
import type {
  Dashboard,
  Member,
  MemberDetail,
  MemberInput,
  MembershipPlan,
  Page,
  PackageDetail,
  PackageInput,
  Payment,
  PaymentInput,
  PaymentPage,
  PlanInput,
  PTPackage,
  PTPlan,
  PTPlanInput,
  PTSession,
  SessionInput,
  SessionStatus,
  Subscription,
  SubscriptionInput,
  Tenant,
  Trainer,
  TrainerDetail,
  TrainerInput,
  User,
  UserInput,
} from './types'

// Query keys -----------------------------------------------------------------
export const keys = {
  me: ['me'] as const,
  dashboard: ['dashboard'] as const,
  tenant: ['tenant'] as const,
  users: ['users'] as const,
  members: (params?: object) => ['members', params ?? {}] as const,
  member: (id: string) => ['member', id] as const,
  trainers: (params?: object) => ['trainers', params ?? {}] as const,
  trainer: (id: string) => ['trainer', id] as const,
  plans: (activeOnly?: boolean) => ['plans', !!activeOnly] as const,
  ptPlans: (activeOnly?: boolean) => ['ptPlans', !!activeOnly] as const,
  payments: (params?: object) => ['payments', params ?? {}] as const,
  packages: (params?: object) => ['packages', params ?? {}] as const,
  pkg: (id: string) => ['package', id] as const,
  sessions: (params?: object) => ['sessions', params ?? {}] as const,
}

/** Invalidate everything that may be affected by a write. Datasets are small
 * enough that broad invalidation is simpler and always correct. */
function useInvalidateAll() {
  const qc = useQueryClient()
  return () =>
    qc.invalidateQueries({
      predicate: (q) => q.queryKey[0] !== 'me',
    })
}

// Dashboard ------------------------------------------------------------------
export function useDashboard() {
  return useQuery({ queryKey: keys.dashboard, queryFn: () => api.get<Dashboard>('/dashboard'), refetchInterval: 60_000 })
}

// Tenant settings ------------------------------------------------------------
export function useUpdateTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: Partial<Pick<Tenant, 'name' | 'timezone' | 'currency' | 'gstin' | 'gst_percent'>>) =>
      api.patch<Tenant>('/settings/tenant', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: keys.me })
      qc.invalidateQueries({ queryKey: keys.dashboard })
    },
  })
}

export function useUsers(opts?: { enabled?: boolean }) {
  return useQuery({
    queryKey: keys.users,
    queryFn: () => api.get<{ items: User[] }>('/users'),
    enabled: opts?.enabled ?? true,
  })
}

export function useCreateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: UserInput) => api.post<User>('/users', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.users }),
  })
}

export function useUpdateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: UserInput }) => api.put<User>(`/users/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.users }),
  })
}

export function useDeleteUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/users/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.users }),
  })
}

// Members --------------------------------------------------------------------
export interface MemberListQuery {
  q?: string
  status?: string
  membership?: string
  page?: number
  page_size?: number
}

export function useMembers(params: MemberListQuery) {
  return useQuery({
    queryKey: keys.members(params),
    queryFn: () => api.get<Page<Member>>('/members', params),
    placeholderData: keepPreviousData,
  })
}

export function useMember(id: string | undefined) {
  return useQuery({
    queryKey: keys.member(id ?? ''),
    queryFn: () => api.get<MemberDetail>(`/members/${id}`),
    enabled: !!id,
  })
}

export function useCreateMember() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: MemberInput) => api.post<Member>('/members', body), onSuccess: invalidate })
}

export function useUpdateMember(id: string) {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: MemberInput) => api.put<Member>(`/members/${id}`, body), onSuccess: invalidate })
}

export function useDeleteMember() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (id: string) => api.delete(`/members/${id}`), onSuccess: invalidate })
}

export function useCreateSubscription(memberId: string) {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: (body: SubscriptionInput) => api.post<Subscription>(`/members/${memberId}/subscriptions`, body),
    onSuccess: invalidate,
  })
}

export function useUpdateSubscription() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: 'active' | 'cancelled' }) =>
      api.patch<Subscription>(`/subscriptions/${id}`, { status }),
    onSuccess: invalidate,
  })
}

// Trainers -------------------------------------------------------------------
export function useTrainers(params: { q?: string; status?: string; page?: number; page_size?: number } = {}) {
  return useQuery({
    queryKey: keys.trainers(params),
    queryFn: () => api.get<Page<Trainer>>('/trainers', params),
    placeholderData: keepPreviousData,
  })
}

export function useTrainer(id: string | undefined) {
  return useQuery({
    queryKey: keys.trainer(id ?? ''),
    queryFn: () => api.get<TrainerDetail>(`/trainers/${id}`),
    enabled: !!id,
  })
}

export function useCreateTrainer() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: TrainerInput) => api.post<Trainer>('/trainers', body), onSuccess: invalidate })
}

export function useUpdateTrainer(id: string) {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: TrainerInput) => api.put<Trainer>(`/trainers/${id}`, body), onSuccess: invalidate })
}

export function useDeleteTrainer() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (id: string) => api.delete(`/trainers/${id}`), onSuccess: invalidate })
}

// Plans ----------------------------------------------------------------------
export function usePlans(activeOnly = false) {
  return useQuery({
    queryKey: keys.plans(activeOnly),
    queryFn: () => api.get<{ items: MembershipPlan[] }>('/plans', activeOnly ? { active: true } : undefined),
    select: (d) => d.items,
  })
}

export function useCreatePlan() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: PlanInput) => api.post<MembershipPlan>('/plans', body), onSuccess: invalidate })
}

export function useUpdatePlan() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: PlanInput }) => api.put<MembershipPlan>(`/plans/${id}`, body),
    onSuccess: invalidate,
  })
}

export function useDeletePlan() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (id: string) => api.delete(`/plans/${id}`), onSuccess: invalidate })
}

export function usePTPlans(activeOnly = false) {
  return useQuery({
    queryKey: keys.ptPlans(activeOnly),
    queryFn: () => api.get<{ items: PTPlan[] }>('/pt/plans', activeOnly ? { active: true } : undefined),
    select: (d) => d.items,
  })
}

export function useCreatePTPlan() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: PTPlanInput) => api.post<PTPlan>('/pt/plans', body), onSuccess: invalidate })
}

export function useUpdatePTPlan() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: PTPlanInput }) => api.put<PTPlan>(`/pt/plans/${id}`, body),
    onSuccess: invalidate,
  })
}

export function useDeletePTPlan() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (id: string) => api.delete(`/pt/plans/${id}`), onSuccess: invalidate })
}

// Payments -------------------------------------------------------------------
export interface PaymentListQuery {
  q?: string
  member_id?: string
  method?: string
  from?: string
  to?: string
  page?: number
  page_size?: number
}

export function usePayments(params: PaymentListQuery) {
  return useQuery({
    queryKey: keys.payments(params),
    queryFn: () => api.get<PaymentPage>('/payments', params),
    placeholderData: keepPreviousData,
  })
}

export function useCreatePayment() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: PaymentInput) => api.post<Payment>('/payments', body), onSuccess: invalidate })
}

export function useDeletePayment() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (id: string) => api.delete(`/payments/${id}`), onSuccess: invalidate })
}

// PT packages ----------------------------------------------------------------
export interface PackageListQuery {
  q?: string
  status?: string
  member_id?: string
  trainer_id?: string
  page?: number
  page_size?: number
}

export function usePackages(params: PackageListQuery) {
  return useQuery({
    queryKey: keys.packages(params),
    queryFn: () => api.get<Page<PTPackage>>('/pt/packages', params),
    placeholderData: keepPreviousData,
  })
}

export function usePackage(id: string | undefined) {
  return useQuery({
    queryKey: keys.pkg(id ?? ''),
    queryFn: () => api.get<PackageDetail>(`/pt/packages/${id}`),
    enabled: !!id,
  })
}

export function useCreatePackage() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: PackageInput) => api.post<PTPackage>('/pt/packages', body), onSuccess: invalidate })
}

export function useUpdatePackage() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: PackageInput }) => api.put<PTPackage>(`/pt/packages/${id}`, body),
    onSuccess: invalidate,
  })
}

export function useDeletePackage() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (id: string) => api.delete(`/pt/packages/${id}`), onSuccess: invalidate })
}

// PT sessions ----------------------------------------------------------------
export interface SessionListQuery {
  q?: string
  status?: string
  member_id?: string
  trainer_id?: string
  package_id?: string
  from?: string
  to?: string
  order?: 'asc' | 'desc'
  page?: number
  page_size?: number
}

export function useSessions(params: SessionListQuery) {
  return useQuery({
    queryKey: keys.sessions(params),
    queryFn: () => api.get<Page<PTSession>>('/pt/sessions', params),
    placeholderData: keepPreviousData,
  })
}

export function useCreateSession() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (body: SessionInput) => api.post<PTSession>('/pt/sessions', body), onSuccess: invalidate })
}

export function useUpdateSession() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: SessionInput }) => api.put<PTSession>(`/pt/sessions/${id}`, body),
    onSuccess: invalidate,
  })
}

export function useSetSessionStatus() {
  const invalidate = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, status, notes }: { id: string; status: SessionStatus; notes?: string }) =>
      api.patch<PTSession>(`/pt/sessions/${id}`, { status, notes }),
    onSuccess: invalidate,
  })
}

export function useDeleteSession() {
  const invalidate = useInvalidateAll()
  return useMutation({ mutationFn: (id: string) => api.delete(`/pt/sessions/${id}`), onSuccess: invalidate })
}
