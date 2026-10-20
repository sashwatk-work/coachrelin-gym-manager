import type { Role, User } from '@/api/types'

export const roles = {
  owner: 'owner',
  receptionist: 'receptionist',
  trainer: 'trainer',
} as const

const permissions = {
  manageUsers: ['owner'],
  gymSettings: ['owner'],
  managePlans: ['owner'],
  manageTrainers: ['owner'],
  deleteMembers: ['owner'],
  deletePayments: ['owner'],
  deletePackages: ['owner'],
  membersWrite: ['owner', 'receptionist'],
  payments: ['owner', 'receptionist'],
  sellPT: ['owner', 'receptionist'],
  viewPlans: ['owner', 'receptionist'],
  viewPayments: ['owner', 'receptionist'],
  viewTrainers: ['owner', 'receptionist'],
  scheduleSessions: ['owner', 'receptionist', 'trainer'],
} as const

export type Permission = keyof typeof permissions

type RoleInput = readonly Role[] | Role | 'cashier' | undefined | null

function roleList(roles: RoleInput): Role[] {
  if (roles == null) return []
  const raw = typeof roles === 'string' ? [roles] : Array.isArray(roles) ? roles : []
  const out: Role[] = []
  for (const r of raw) {
    const n = r === 'cashier' ? 'receptionist' : r
    if (n === 'owner' || n === 'receptionist' || n === 'trainer') {
      if (!out.includes(n)) out.push(n)
    }
  }
  return out
}

export function hasRole(roles: RoleInput, role: Role): boolean {
  return roleList(roles).includes(role)
}

export function can(roles: RoleInput, perm: Permission): boolean {
  return roleList(roles).some((r) => (permissions[perm] as readonly string[]).includes(r))
}

/** Trainer without owner/receptionist — lists stay locked to their own clients. */
export function isTrainerOnly(roles: RoleInput): boolean {
  return hasRole(roles, 'trainer') && !hasRole(roles, 'owner') && !hasRole(roles, 'receptionist')
}

export function roleLabel(role: Role | undefined): string {
  switch (role) {
    case 'owner':
      return 'Owner'
    case 'receptionist':
      return 'Receptionist'
    case 'trainer':
      return 'Trainer'
    default:
      return 'Staff'
  }
}

export function rolesLabel(roles: RoleInput): string {
  const list = roleList(roles)
  if (!list.length) return 'Staff'
  return list.map((r) => roleLabel(r)).join(' · ')
}

export function consoleLabel(roles: RoleInput): string {
  if (hasRole(roles, 'owner')) return 'Owner console'
  if (hasRole(roles, 'receptionist')) return 'Front desk'
  if (hasRole(roles, 'trainer')) return 'Trainer'
  return 'Console'
}

export function normalizeUser(u: User & { role?: string }): User {
  const roles = roleList(u.roles?.length ? u.roles : (u.role as Role | undefined))
  return { ...u, roles }
}
