// Types mirror the Go JSON models in backend/internal/store/models.go

export interface Tenant {
  id: string
  name: string
  slug: string
  timezone: string
  currency: string
  gstin: string | null
  gst_percent: number
  created_at: string
}

export type Role = 'owner' | 'receptionist' | 'trainer'

export interface User {
  id: string
  tenant_id: string
  email: string
  full_name: string
  roles: Role[]
  trainer_id: string | null
  trainer_name?: string | null
  created_at: string
}

export interface UserInput {
  full_name: string
  email?: string
  password?: string
  roles: Role[]
  trainer_id?: string | null
}

export interface AuthResponse {
  user: User
  tenant: Tenant
}

export type MemberStatus = 'active' | 'inactive' | 'frozen'
export type MembershipStatus = 'active' | 'expiring' | 'expired' | 'none'
export type SubscriptionStatus = 'active' | 'expired' | 'cancelled'
export type PaymentMethod = 'cash' | 'card' | 'upi' | 'bank_transfer' | 'other'
export type PackageStatus = 'active' | 'completed' | 'expired' | 'cancelled'
export type SessionStatus = 'scheduled' | 'completed' | 'cancelled' | 'no_show'

export interface Member {
  id: string
  full_name: string
  email: string | null
  phone: string | null
  date_of_birth: string | null
  gender: string | null
  joined_at: string
  status: MemberStatus
  emergency_contact: string | null
  notes: string
  created_at: string
  updated_at: string
  current_plan_name: string | null
  membership_end_date: string | null
  membership_status: MembershipStatus
  outstanding_dues: number
}

export interface MemberInput {
  full_name: string
  email?: string | null
  phone?: string | null
  date_of_birth?: string | null
  gender?: string | null
  joined_at?: string
  status?: MemberStatus
  emergency_contact?: string | null
  notes?: string
}

export interface Subscription {
  id: string
  member_id: string
  plan_id: string
  plan_name: string
  start_date: string
  end_date: string
  price: number
  paid: number
  status: SubscriptionStatus
  created_at: string
}

export interface SubscriptionInput {
  plan_id: string
  start_date?: string
  end_date?: string
  price?: number
}

export interface Payment {
  id: string
  member_id: string
  member_name: string
  subscription_id: string | null
  pt_package_id: string | null
  amount: number
  method: PaymentMethod
  paid_at: string
  notes: string
  created_at: string
  reference_label: string
}

export interface PaymentInput {
  member_id: string
  subscription_id?: string | null
  pt_package_id?: string | null
  amount: number
  method: PaymentMethod
  paid_at?: string
  notes?: string
}

export interface Trainer {
  id: string
  full_name: string
  email: string | null
  phone: string | null
  specialties: string[]
  active: boolean
  created_at: string
  updated_at: string
  active_clients: number
  upcoming_sessions: number
}

export interface TrainerInput {
  full_name: string
  email?: string | null
  phone?: string | null
  specialties?: string[]
  active?: boolean
}

export interface MembershipPlan {
  id: string
  name: string
  duration_days: number
  price: number
  description: string
  active: boolean
  created_at: string
  updated_at: string
  active_subscriptions: number
}

export interface PlanInput {
  name: string
  duration_days: number
  price: number
  description?: string
  active?: boolean
}

export interface PTPlan {
  id: string
  name: string
  sessions: number
  validity_days: number | null
  price: number
  description: string
  active: boolean
  created_at: string
  updated_at: string
  active_packages: number
}

export interface PTPlanInput {
  name: string
  sessions: number
  validity_days?: number | null
  price: number
  description?: string
  active?: boolean
}

export interface PTPackage {
  id: string
  member_id: string
  member_name: string
  trainer_id: string
  trainer_name: string
  pt_plan_id: string | null
  pt_plan_name: string | null
  name: string
  total_sessions: number
  sessions_used: number
  sessions_scheduled: number
  sessions_remaining: number
  price: number
  paid: number
  purchased_at: string
  expires_at: string | null
  status: PackageStatus
  created_at: string
  updated_at: string
}

export interface PackageInput {
  member_id?: string
  trainer_id: string
  pt_plan_id?: string | null
  name?: string
  total_sessions?: number
  price?: number
  purchased_at?: string
  expires_at?: string | null
  status?: PackageStatus
}

export interface PTSession {
  id: string
  package_id: string
  package_name: string
  member_id: string
  member_name: string
  trainer_id: string
  trainer_name: string
  scheduled_at: string
  duration_minutes: number
  status: SessionStatus
  notes: string
  created_at: string
  updated_at: string
}

export interface SessionInput {
  package_id: string
  trainer_id?: string | null
  scheduled_at: string
  duration_minutes?: number
  status?: SessionStatus
  notes?: string
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface PaymentPage extends Page<Payment> {
  total_amount: number
}

export interface MemberDetail extends Member {
  subscriptions: Subscription[]
  packages: PTPackage[]
  payments: Payment[]
  sessions: PTSession[]
}

export interface TrainerDetail extends Trainer {
  packages: PTPackage[]
  sessions: PTSession[]
}

export interface PackageDetail extends PTPackage {
  sessions: PTSession[]
  payments: Payment[]
}

export interface ExpiringMembership {
  member_id: string
  member_name: string
  plan_name: string
  end_date: string
  days_left: number
}

export interface RevenuePoint {
  month: string
  amount: number
}

export interface Dashboard {
  today: string
  total_members: number
  active_members: number
  active_memberships: number
  expiring_soon: number
  expired_memberships: number
  new_members_this_month: number
  revenue_this_month: number
  revenue_last_month: number
  outstanding_dues: number
  active_trainers: number
  active_pt_packages: number
  sessions_today: number
  sessions_this_week: number
  sessions_completed_this_month: number
  expiring_memberships: ExpiringMembership[]
  today_sessions: PTSession[]
  recent_payments: Payment[]
  revenue_trend: RevenuePoint[]
}
