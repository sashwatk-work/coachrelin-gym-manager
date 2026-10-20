// Coach Relin Fitness bill: ink masthead, CRF mark, Barlow Condensed + Manrope (OFL).
import { jsPDF } from 'jspdf'
import type { Bill } from '@/lib/bill'
import logoUrl from '@/assets/bill/logo.png?url'
import barlowExtraUrl from '@/assets/fonts/BarlowCondensed-ExtraBold.ttf?url'
import barlowBoldUrl from '@/assets/fonts/BarlowCondensed-Bold.ttf?url'
import manropeRegularUrl from '@/assets/fonts/Manrope-Regular.ttf?url'
import manropeMediumUrl from '@/assets/fonts/Manrope-Medium.ttf?url'
import manropeExtraUrl from '@/assets/fonts/Manrope-ExtraBold.ttf?url'

const INK: [number, number, number] = [17, 17, 15]
const INK_SOFT: [number, number, number] = [55, 55, 49]
const MUTED: [number, number, number] = [102, 99, 92]
const LINE: [number, number, number] = [196, 192, 182]
const SIGNAL: [number, number, number] = [233, 242, 28]
const PAPER: [number, number, number] = [243, 240, 233]
const ON_INK: [number, number, number] = [196, 193, 182]

const DISPLAY = 'BarlowCondensed'
const BODY = 'Manrope'
const MARGIN = 18
const BRAND = 'Coach Relin Fitness'
const TAGLINE = 'Every body is different'
const SITE = 'coachrelin.com'

type FontPack = {
  displayExtra: string
  displayBold: string
  body: string
  bodyMedium: string
  bodyExtra: string
}

type Assets = {
  logo: string | null
  fonts: FontPack | null
}

let assetsPromise: Promise<Assets> | null = null

export async function loadBillAssets(): Promise<Assets> {
  if (!assetsPromise) {
    assetsPromise = (async () => {
      try {
        const [logo, displayExtra, displayBold, body, bodyMedium, bodyExtra] = await Promise.all([
          fetchAsDataUrl(logoUrl).catch(() => null),
          fetchAsBase64(barlowExtraUrl),
          fetchAsBase64(barlowBoldUrl),
          fetchAsBase64(manropeRegularUrl),
          fetchAsBase64(manropeMediumUrl),
          fetchAsBase64(manropeExtraUrl),
        ])
        return {
          logo,
          fonts: { displayExtra, displayBold, body, bodyMedium, bodyExtra },
        }
      } catch {
        return { logo: null, fonts: null }
      }
    })()
  }
  return assetsPromise
}

export function renderBillPdf(bill: Bill, assets: Assets): jsPDF {
  const doc = new jsPDF({ unit: 'mm', format: 'a4' })
  const pageW = doc.internal.pageSize.getWidth()
  const pageH = doc.internal.pageSize.getHeight()
  const right = pageW - MARGIN
  const fontsReady = registerFonts(doc, assets.fonts)

  const studio = studioLine(bill.gym)
  const gstin = bill.gstin?.trim() || null
  const gstPercent = bill.gstPercent && bill.gstPercent > 0 ? bill.gstPercent : 0
  const kind = bill.title.toLowerCase().includes('receipt') ? 'RECEIPT' : 'BILL'
  const issued = new Intl.DateTimeFormat('en-IN', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  }).format(bill.issuedAt)

  const dueTotal = bill.lines.reduce((a, l) => a + Math.max(0, l.amount - l.paid), 0)
  const chargedTotal = bill.lines.reduce((a, l) => a + l.amount, 0)

  const dueW = 46
  const chargedW = 40
  const dueCx = right - dueW / 2
  const chargedCx = right - dueW - chargedW / 2
  const chargedLeft = right - dueW - chargedW

  fillPage(doc, pageW, pageH)
  let y = drawMasthead(doc, {
    pageW,
    right,
    dueCx,
    logo: assets.logo,
    fontsReady,
    studio,
    gstin,
    gstPercent,
    kind,
    issued,
    compact: false,
  })

  y += 14
  setBody(doc, fontsReady, 'medium', 7.5)
  doc.setTextColor(...MUTED)
  doc.setCharSpace(0.45)
  doc.text('BILLED TO', MARGIN, y)
  doc.setCharSpace(0)
  doc.text('ISSUED', dueCx, y, { align: 'center' })

  y += 8
  setDisplay(doc, fontsReady, 'bold', 22)
  doc.setTextColor(...INK)
  const nameLines = doc.splitTextToSize(bill.memberName, chargedLeft - MARGIN - 6) as string[]
  doc.text(nameLines, MARGIN, y)
  setBody(doc, fontsReady, 'normal', 9)
  doc.setTextColor(...INK_SOFT)
  doc.text(issued, dueCx, y, { align: 'center' })
  y += Math.max(8, nameLines.length * 8)

  const contact = [bill.memberPhone, bill.memberEmail].filter(Boolean).join('   ·   ')
  if (contact) {
    y += 2
    setBody(doc, fontsReady, 'normal', 9)
    doc.setTextColor(...MUTED)
    doc.text(contact, MARGIN, y)
    y += 4
  }

  y += 10
  y = rule(doc, MARGIN, y, right)

  y += 7
  setBody(doc, fontsReady, 'medium', 7)
  doc.setTextColor(...MUTED)
  doc.setCharSpace(0.45)
  doc.text('DESCRIPTION', MARGIN, y)
  doc.setCharSpace(0)
  doc.text('CHARGED', chargedCx, y, { align: 'center' })
  doc.text('DUE', dueCx, y, { align: 'center' })
  y += 3
  y = rule(doc, MARGIN, y, right, LINE)
  if (gstPercent > 0) {
    y += 5
    setBody(doc, fontsReady, 'medium', 7)
    doc.setTextColor(...MUTED)
    doc.setCharSpace(0.45)
    doc.text(`INCL. GST @ ${formatRate(gstPercent)}%`, MARGIN, y)
    doc.setCharSpace(0)
    y += 2
    y = rule(doc, MARGIN, y, right, LINE)
  }

  const colW = chargedLeft - MARGIN - 4
  if (bill.lines.length === 0) {
    y += 10
    setBody(doc, fontsReady, 'normal', 10)
    doc.setTextColor(...MUTED)
    doc.text('Nothing billed on this account.', MARGIN, y)
    y += 6
  }

  for (const line of bill.lines) {
    const due = Math.max(0, line.amount - line.paid)
    const labels = doc.splitTextToSize(line.label, colW) as string[]
    const block = 7 + labels.length * 5 + (line.detail ? 4.5 : 0)
    if (y + block > 248) {
      y = newPage(doc, {
        pageW,
        pageH,
        right,
        dueCx,
        logo: assets.logo,
        fontsReady,
        studio,
        gstin,
        gstPercent,
        kind,
        issued,
      })
    }
    y += 8
    setBody(doc, fontsReady, 'medium', 10.5)
    doc.setTextColor(...INK)
    doc.text(labels, MARGIN, y)
    drawMoney(doc, line.amount, chargedCx, y, bill.currency)
    if (due > 0) {
      setBody(doc, fontsReady, 'extra', 10.5)
      drawMoney(doc, due, dueCx, y, bill.currency)
    } else {
      setBody(doc, fontsReady, 'medium', 10.5)
      doc.setTextColor(...INK_SOFT)
      doc.text('Paid', dueCx, y, { align: 'center' })
    }
    y += (labels.length - 1) * 5
    if (line.detail) {
      y += 4.5
      setBody(doc, fontsReady, 'normal', 8)
      doc.setTextColor(...MUTED)
      doc.text(line.detail, MARGIN, y)
    }
    y += 5
    y = rule(doc, MARGIN, y, right, LINE)
  }

  if (y > (gstPercent > 0 ? 200 : 228)) {
    y = newPage(doc, {
      pageW,
      pageH,
      right,
      dueCx,
      logo: assets.logo,
      fontsReady,
      studio,
      gstin,
      gstPercent,
      kind,
      issued,
    })
  }

  if (gstPercent > 0) {
    y += 10
    const split = gstSplit(chargedTotal, gstPercent)
    setBody(doc, fontsReady, 'medium', 7)
    doc.setTextColor(...MUTED)
    doc.setCharSpace(0.45)
    doc.text(`GST @ ${formatRate(gstPercent)}%`, MARGIN, y)
    doc.setCharSpace(0)
    doc.text('INCLUSIVE', dueCx, y, { align: 'center' })
    y += 3
    y = rule(doc, MARGIN, y, right, LINE)
    const taxRows: Array<{ label: string; amount: number }> = [
      { label: 'Taxable value', amount: split.taxable },
      { label: `CGST @ ${formatRate(split.half)}%`, amount: split.cgst },
      { label: `SGST @ ${formatRate(split.half)}%`, amount: split.sgst },
    ]
    for (const row of taxRows) {
      y += 7
      setBody(doc, fontsReady, 'medium', 10)
      doc.setTextColor(...INK)
      doc.text(row.label, MARGIN, y)
      setBody(doc, fontsReady, 'medium', 10)
      drawMoney(doc, row.amount, chargedCx, y, bill.currency)
    }
    y += 5
    y = rule(doc, MARGIN, y, right, LINE)
  }

  y += 10
  const settled = dueTotal <= 0
  const barY = y + 12.2
  doc.setFillColor(...(settled ? SIGNAL : INK))
  doc.rect(MARGIN, y, right - MARGIN, 18, 'F')
  setDisplay(doc, fontsReady, 'extrabold', 12)
  doc.setCharSpace(1.1)
  doc.setTextColor(...(settled ? INK : SIGNAL))
  doc.text(settled ? 'SETTLED' : 'DUE NOW', MARGIN + 5, y + 11.5)
  doc.setCharSpace(0)
  setBody(doc, fontsReady, 'extra', 12)
  doc.setTextColor(...(settled ? INK : SIGNAL))
  drawMoney(doc, chargedTotal, chargedCx, barY, bill.currency)
  if (settled) {
    setBody(doc, fontsReady, 'extra', 12)
    doc.text('Paid', dueCx, barY, { align: 'center' })
  } else {
    drawMoney(doc, dueTotal, dueCx, barY, bill.currency)
  }
  y += 18

  if (bill.payments && bill.payments.length > 0) {
    if (y > 236) {
      y = newPage(doc, {
        pageW,
        pageH,
        right,
        dueCx,
        logo: assets.logo,
        fontsReady,
        studio,
        gstin,
        gstPercent,
        kind,
        issued,
      })
    }
    y += 14
    setDisplay(doc, fontsReady, 'bold', 12)
    doc.setTextColor(...INK)
    doc.text('Payments', MARGIN, y)
    y += 2
    for (const p of bill.payments) {
      y += 7
      if (y > 262) {
        y = newPage(doc, {
        pageW,
        pageH,
        right,
        dueCx,
        logo: assets.logo,
        fontsReady,
        studio,
        gstin,
        gstPercent,
        kind,
        issued,
      })
      }
      setBody(doc, fontsReady, 'normal', 9)
      doc.setTextColor(...INK_SOFT)
      const payLabel = doc.splitTextToSize(`${p.when}  ·  ${p.label}`, chargedLeft - MARGIN - 4) as string[]
      doc.text(payLabel[0], MARGIN, y)
      setBody(doc, fontsReady, 'medium', 9)
      doc.setTextColor(...INK)
      drawMoney(doc, p.amount, dueCx, y, bill.currency)
    }
  }

  if (bill.notes) {
    y += 12
    setBody(doc, fontsReady, 'normal', 9)
    doc.setTextColor(...MUTED)
    doc.text(doc.splitTextToSize(bill.notes, right - MARGIN) as string[], MARGIN, y)
  }

  stampFooters(doc, fontsReady, pageW, gstin, gstPercent)
  return doc
}

function studioLine(gym: string): string | null {
  const n = gym.trim()
  if (!n) return null
  if (n.toLowerCase() === BRAND.toLowerCase()) return null
  return n
}

function round2(n: number): number {
  return Math.round(n * 100) / 100
}

function formatRate(n: number): string {
  if (Number.isInteger(n)) return String(n)
  return n.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
}

function gstSplit(inclusive: number, percent: number) {
  const taxable = round2(inclusive / (1 + percent / 100))
  const gst = round2(inclusive - taxable)
  const cgst = round2(gst / 2)
  return { taxable, gst, cgst, sgst: round2(gst - cgst), half: percent / 2 }
}

function formatFigure(amount: number): string {
  const fraction = Number.isInteger(amount) ? 0 : 2
  return new Intl.NumberFormat('en-IN', {
    minimumFractionDigits: fraction,
    maximumFractionDigits: fraction,
  }).format(amount)
}

function currencySymbol(currency: string): string {
  if (currency === 'INR') return '₹'
  try {
    const parts = new Intl.NumberFormat('en-IN', { style: 'currency', currency, currencyDisplay: 'code' }).formatToParts(0)
    return parts.find((p) => p.type === 'currency')?.value ?? currency
  } catch {
    return currency
  }
}

/** Centers ₹ + digits as one unit on cx so headers, Paid, and amounts share a midline. */
function drawMoney(doc: jsPDF, amount: number, cx: number, y: number, currency: string) {
  const figure = formatFigure(amount)
  const symbol = currencySymbol(currency)
  const figW = doc.getTextWidth(figure)
  const gap = 0.9
  const symW = 3.6
  const total = symW + gap + figW
  const left = cx - total / 2
  doc.text(symbol, left + symW, y, { align: 'right' })
  doc.text(figure, left + symW + gap, y)
}

type Masthead = {
  pageW: number
  right: number
  dueCx: number
  logo: string | null
  fontsReady: boolean
  studio: string | null
  gstin: string | null
  gstPercent: number
  kind: string
  issued: string
  compact: boolean
}

function fillPage(doc: jsPDF, pageW: number, pageH: number) {
  doc.setFillColor(...PAPER)
  doc.rect(0, 0, pageW, pageH, 'F')
}

function drawMasthead(doc: jsPDF, m: Masthead): number {
  const h = m.compact ? 22 : 52
  doc.setFillColor(...INK)
  doc.rect(0, 0, m.pageW, h, 'F')
  doc.setFillColor(...SIGNAL)
  doc.rect(0, h, m.pageW, 3.2, 'F')

  const mark = m.compact ? 12 : 26
  const markY = m.compact ? 5 : 12
  if (m.logo) {
    doc.addImage(m.logo, 'PNG', MARGIN, markY, mark, mark)
  } else {
    doc.setFillColor(...SIGNAL)
    doc.rect(MARGIN, markY, mark, mark, 'F')
  }

  const textX = MARGIN + mark + 6
  if (m.compact) {
    setDisplay(doc, m.fontsReady, 'extrabold', 13)
    doc.setTextColor(...SIGNAL)
    doc.text('COACH RELIN FITNESS', textX, 14)
    setBody(doc, m.fontsReady, 'medium', 8)
    doc.setTextColor(...ON_INK)
    doc.text(m.kind, m.dueCx, 14, { align: 'center' })
    return h + 3.2
  }

  setDisplay(doc, m.fontsReady, 'extrabold', 26)
  doc.setTextColor(...SIGNAL)
  doc.setCharSpace(0.35)
  doc.text('COACH RELIN', textX, 24)
  doc.setCharSpace(0)
  setDisplay(doc, m.fontsReady, 'bold', 11)
  doc.setCharSpace(2.4)
  doc.text('FITNESS', textX, 32)
  doc.setCharSpace(0)

  const leftBits = [
    m.studio,
    m.gstPercent > 0 ? `GST ${formatRate(m.gstPercent)}% inclusive` : null,
  ].filter(Boolean) as string[]
  if (leftBits.length) {
    setBody(doc, m.fontsReady, 'normal', 8.5)
    doc.setTextColor(...ON_INK)
    doc.text(leftBits.join('  ·  '), textX, 40)
  }

  const stampW = 34
  const stampH = 11
  const stampX = m.dueCx - stampW / 2
  doc.setFillColor(...SIGNAL)
  doc.rect(stampX, 14, stampW, stampH, 'F')
  setDisplay(doc, m.fontsReady, 'extrabold', 12)
  doc.setTextColor(...INK)
  doc.setCharSpace(1.3)
  doc.text(m.kind, m.dueCx, 21.4, { align: 'center' })
  doc.setCharSpace(0)
  setBody(doc, m.fontsReady, 'normal', 8)
  doc.setTextColor(...ON_INK)
  doc.text(m.issued, m.dueCx, 32, { align: 'center' })
  if (m.gstin) {
    setBody(doc, m.fontsReady, 'medium', 7.5)
    doc.setTextColor(...ON_INK)
    doc.text(`GSTIN ${m.gstin}`, m.dueCx, 40, { align: 'center' })
  }

  return h + 3.2
}

function newPage(
  doc: jsPDF,
  m: Omit<Masthead, 'compact'> & { pageH: number },
): number {
  stampFooters(doc, m.fontsReady, m.pageW, m.gstin, m.gstPercent)
  doc.addPage()
  fillPage(doc, m.pageW, m.pageH)
  return drawMasthead(doc, { ...m, compact: true }) + 12
}

function stampFooters(doc: jsPDF, fontsReady: boolean, pageW: number, gstin: string | null, gstPercent: number) {
  const pages = doc.getNumberOfPages()
  const right = pageW - MARGIN
  const gstBits = [gstin ? `GSTIN ${gstin}` : null, gstPercent > 0 ? `GST ${formatRate(gstPercent)}%` : null].filter(Boolean)
  const left = gstBits.length ? gstBits.join('  ·  ') : SITE
  for (let i = 1; i <= pages; i++) {
    doc.setPage(i)
    doc.setDrawColor(...LINE)
    doc.setLineWidth(0.2)
    doc.line(MARGIN, 278, right, 278)
    setBody(doc, fontsReady, 'normal', 7.5)
    doc.setTextColor(...MUTED)
    doc.text(left, MARGIN, 285)
    doc.text(gstin ? SITE : TAGLINE, pageW / 2, 285, { align: 'center' })
    doc.text(`${i} / ${pages}`, right, 285, { align: 'right' })
  }
}

function rule(doc: jsPDF, x1: number, y: number, x2: number, color: [number, number, number] = INK): number {
  doc.setDrawColor(...color)
  doc.setLineWidth(color === INK ? 0.35 : 0.2)
  doc.line(x1, y, x2, y)
  return y
}

function setDisplay(doc: jsPDF, fontsReady: boolean, style: 'extrabold' | 'bold', size: number) {
  doc.setFont(fontsReady ? DISPLAY : 'helvetica', fontsReady ? style : 'bold')
  doc.setFontSize(size)
}

function setBody(doc: jsPDF, fontsReady: boolean, style: 'normal' | 'medium' | 'extra', size: number) {
  if (fontsReady) {
    doc.setFont(BODY, style)
  } else {
    doc.setFont('helvetica', style === 'normal' ? 'normal' : 'bold')
  }
  doc.setFontSize(size)
}

function registerFonts(doc: jsPDF, fonts: FontPack | null): boolean {
  if (!fonts) return false
  try {
    doc.addFileToVFS('BarlowCondensed-ExtraBold.ttf', fonts.displayExtra)
    doc.addFont('BarlowCondensed-ExtraBold.ttf', DISPLAY, 'extrabold')
    doc.addFileToVFS('BarlowCondensed-Bold.ttf', fonts.displayBold)
    doc.addFont('BarlowCondensed-Bold.ttf', DISPLAY, 'bold')
    doc.addFileToVFS('Manrope-Regular.ttf', fonts.body)
    doc.addFont('Manrope-Regular.ttf', BODY, 'normal')
    doc.addFileToVFS('Manrope-Medium.ttf', fonts.bodyMedium)
    doc.addFont('Manrope-Medium.ttf', BODY, 'medium')
    doc.addFileToVFS('Manrope-ExtraBold.ttf', fonts.bodyExtra)
    doc.addFont('Manrope-ExtraBold.ttf', BODY, 'extra')
    return true
  } catch {
    return false
  }
}

async function fetchAsDataUrl(url: string): Promise<string> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`asset ${res.status}`)
  const blob = await res.blob()
  return await new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(blob)
  })
}

async function fetchAsBase64(url: string): Promise<string> {
  const res = await fetch(url)
  if (!res.ok) throw new Error(`font ${res.status}`)
  const buf = await res.arrayBuffer()
  const bytes = new Uint8Array(buf)
  const chunks: string[] = []
  for (let i = 0; i < bytes.length; i += 0x8000) {
    chunks.push(String.fromCharCode(...bytes.subarray(i, i + 0x8000)))
  }
  return btoa(chunks.join(''))
}
