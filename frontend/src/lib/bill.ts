import type { MemberDetail, Payment, Tenant } from '@/api/types'
import { formatDate, formatDateTime, formatMoney, titleCase } from '@/lib/format'
import { loadBillAssets, renderBillPdf } from '@/lib/bill-pdf'

export const BRAND_NAME = 'Coach Relin Fitness'
export const BRAND_TAGLINE = 'Every body is different'
export const BRAND_SITE = 'coachrelin.com'

export interface BillLine {
  label: string
  detail?: string
  amount: number
  paid: number
}

export interface Bill {
  title: string
  gym: string
  gstin?: string | null
  gstPercent?: number | null
  memberName: string
  memberPhone?: string | null
  memberEmail?: string | null
  issuedAt: Date
  currency: string
  lines: BillLine[]
  payments?: Array<{ when: string; label: string; amount: number }>
  notes?: string
}

export function whatsAppNumber(phone: string | null | undefined): string | null {
  if (!phone) return null
  let d = phone.replace(/\D/g, '')
  if (d.startsWith('00')) d = d.slice(2)
  if (d.startsWith('0') && d.length === 11) d = d.slice(1)
  if (d.length === 10) d = `91${d}`
  if (d.length < 11 || d.length > 15) return null
  return d
}

export function whatsAppBillUrl(phone: string | null | undefined, text: string): string | null {
  const n = whatsAppNumber(phone)
  if (!n) return null
  return `https://wa.me/${n}?text=${encodeURIComponent(text)}`
}

export function billWhatsAppText(bill: Bill): string {
  const money = (n: number) => formatMoney(n, bill.currency)
  const due = bill.lines.reduce((a, l) => a + Math.max(0, l.amount - l.paid), 0)
  const charged = bill.lines.reduce((a, l) => a + l.amount, 0)
  const lines = bill.lines.map((l) => {
    const rest = Math.max(0, l.amount - l.paid)
    return `• ${l.label}${l.detail ? ` (${l.detail})` : ''} — ${money(l.amount)}${rest > 0 ? ` · due ${money(rest)}` : ' · paid'}`
  })
  const studio = bill.gym.trim() && bill.gym.trim().toLowerCase() !== BRAND_NAME.toLowerCase() ? bill.gym.trim() : null
  const gstin = bill.gstin?.trim() ? `GSTIN ${bill.gstin.trim()}` : null
  const gst = bill.gstPercent && bill.gstPercent > 0 ? `GST ${bill.gstPercent}% (inclusive)` : null
  return [
    BRAND_NAME,
    studio,
    gstin,
    gst,
    bill.title,
    '',
    bill.memberName,
    ...lines,
    '',
    due > 0 ? `Outstanding: ${money(due)}` : `Total: ${money(charged)} · paid`,
    '',
    BRAND_TAGLINE,
    BRAND_SITE,
  ]
    .filter((s): s is string => typeof s === 'string')
    .join('\n')
}

export function memberStatement(member: MemberDetail, tenant: Tenant): Bill {
  const lines: BillLine[] = []
  for (const s of member.subscriptions) {
    if (s.status === 'cancelled') continue
    lines.push({
      label: s.plan_name,
      detail: `${formatDate(s.start_date)} – ${formatDate(s.end_date)}`,
      amount: s.price,
      paid: s.paid,
    })
  }
  for (const p of member.packages) {
    if (p.status === 'cancelled') continue
    lines.push({
      label: p.name,
      detail: `PT · ${p.trainer_name}`,
      amount: p.price,
      paid: p.paid,
    })
  }
  return {
    title: 'Bill',
    gym: tenant.name,
    gstin: tenant.gstin,
    gstPercent: tenant.gst_percent,
    memberName: member.full_name,
    memberPhone: member.phone,
    memberEmail: member.email,
    issuedAt: new Date(),
    currency: tenant.currency,
    lines,
    payments: member.payments.slice(0, 8).map((p) => ({
      when: formatDateTime(p.paid_at),
      label: p.reference_label,
      amount: p.amount,
    })),
  }
}

export function paymentReceipt(payment: Payment, tenant: Tenant, memberPhone?: string | null): Bill {
  return {
    title: 'Payment receipt',
    gym: tenant.name,
    gstin: tenant.gstin,
    gstPercent: tenant.gst_percent,
    memberName: payment.member_name,
    memberPhone,
    issuedAt: new Date(payment.paid_at),
    currency: tenant.currency,
    lines: [
      {
        label: payment.reference_label || 'Payment',
        detail: `${titleCase(payment.method)}${payment.notes ? ` · ${payment.notes}` : ''}`,
        amount: payment.amount,
        paid: payment.amount,
      },
    ],
  }
}

export async function downloadBillPdf(bill: Bill) {
  const assets = await loadBillAssets()
  const doc = renderBillPdf(bill, assets)
  const slug = bill.memberName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
  const day = bill.issuedAt.toISOString().slice(0, 10)
  doc.save(`${slug || 'bill'}-${day}.pdf`)
}

export function openWhatsAppBill(phone: string | null | undefined, bill: Bill): boolean {
  const url = whatsAppBillUrl(phone, billWhatsAppText(bill))
  if (!url) return false
  window.open(url, '_blank', 'noopener,noreferrer')
  return true
}
