import type { FieldValues, Path, UseFormSetError } from 'react-hook-form'
import { z } from 'zod'
import { ApiError } from '@/api/client'

/** Numeric form field kept as a string in the form state, validated as a number. */
export function numField(opts: { min?: number; integer?: boolean; required?: boolean } = {}) {
  const { min, integer, required = true } = opts
  let s = z.string().trim()
  if (required) s = s.min(1, 'Required')
  return s
    .refine((v) => v === '' || !Number.isNaN(Number(v)), 'Must be a number')
    .refine((v) => v === '' || !integer || Number.isInteger(Number(v)), 'Must be a whole number')
    .refine((v) => v === '' || min === undefined || Number(v) >= min, `Must be at least ${min}`)
}

export const optionalStr = z.string().trim().optional().or(z.literal(''))
export const optionalEmail = z
  .string()
  .trim()
  .optional()
  .or(z.literal(''))
  .refine((v) => !v || /.+@.+/.test(v), 'Enter a valid email')

export const dateStr = z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Pick a date')
export const optionalDateStr = z.string().regex(/^(\d{4}-\d{2}-\d{2})?$/, 'Pick a date')

export function nullIfEmpty(v: string | undefined | null): string | null {
  const t = (v ?? '').trim()
  return t === '' ? null : t
}

/** Pushes server-side field errors into react-hook-form; returns the general message if any. */
export function applyApiErrors<T extends FieldValues>(e: unknown, setError: UseFormSetError<T>): string {
  if (e instanceof ApiError) {
    if (e.fields) {
      for (const [k, msg] of Object.entries(e.fields)) {
        setError(k as Path<T>, { message: msg })
      }
      return ''
    }
    return e.message
  }
  return e instanceof Error ? e.message : String(e)
}
