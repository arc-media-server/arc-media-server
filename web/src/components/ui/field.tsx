import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/** The look of every text box and drop-down in a form. Add sizes with cn(inputClass, 'w-20'). */
export const inputClass =
  'h-10 w-full rounded-xl bg-surface-2/80 px-3 text-sm text-foreground ring-1 ring-border outline-none transition-shadow placeholder:text-subtle focus:ring-2 focus:ring-accent [color-scheme:inherit] [&>option]:bg-surface';

/** A small uppercase label above a form control. */
export function FieldLabel({ children, className }: { children: ReactNode; className?: string }) {
  return <span className={cn('mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle', className)}>{children}</span>;
}

/** A labelled form control. */
export function Field({ label, children, className }: { label: string; children: ReactNode; className?: string }) {
  return (
    <label className={cn('block', className)}>
      <FieldLabel>{label}</FieldLabel>
      {children}
    </label>
  );
}
