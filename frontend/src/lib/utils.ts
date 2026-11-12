import type { ClassValue } from 'clsx'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Merges Tailwind CSS class names, resolving conflicts intelligently.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}

/** The message of a rejected binding call or other thrown value. */
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
