import { useTranslation } from 'react-i18next';
import type { ReactNode } from 'react';
import { motion } from 'motion/react';
import { cn } from '@/lib/utils';
import { useQueryClient } from '@tanstack/react-query';
import { Button } from './ui/button';

/** Wraps each page with a gentle fade-and-rise entrance. */
export function Page({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.35, ease: [0.22, 1, 0.36, 1] }}
      className={cn('min-h-full', className)}
    >
      {children}
    </motion.div>
  );
}

export function PageHeader({ title, subtitle, actions }: { title: string; subtitle?: ReactNode; actions?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4 px-6 pb-6 pt-10 md:px-10 md:pt-14">
      <div>
        <h1 className="text-3xl font-bold md:text-4xl">{title}</h1>
        {subtitle && <p className="mt-1.5 text-sm text-muted">{subtitle}</p>}
      </div>
      {actions && <div className="flex max-w-full flex-wrap items-center gap-2">{actions}</div>}
    </header>
  );
}

export function EmptyState({ icon, title, children }: { icon: ReactNode; title: string; children?: ReactNode }) {
  return (
    <div className="mx-auto flex max-w-md flex-col items-center px-6 py-24 text-center">
      <div className="mb-5 flex size-16 items-center justify-center rounded-3xl bg-surface-2 text-muted [&_svg]:size-7">
        {icon}
      </div>
      <h2 className="text-xl font-semibold">{title}</h2>
      {children && <div className="mt-2 text-sm leading-relaxed text-muted">{children}</div>}
    </div>
  );
}

export function ErrorNotice({ error, onRetry, busy = false }: { error: unknown; onRetry?: () => void; busy?: boolean }) {
  const { t: tr } = useTranslation('common');
  if (!error) return null;
  return (
    <div role="alert" className="flex flex-wrap items-center gap-3 rounded-xl bg-warning/10 p-3 text-sm text-warning">
      <span className="min-w-0 flex-1 break-words">{error instanceof Error ? error.message : String(error)}</span>
      {onRetry && <Button size="sm" variant="secondary" disabled={busy} onClick={onRetry}>{busy ? tr('actions.retrying') : tr('actions.retry')}</Button>}
    </div>
  );
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const { t: tr } = useTranslation('common');
  const qc = useQueryClient();
  return (
    <div className="mx-auto max-w-md px-6 py-24 text-center">
      <h2 className="text-lg font-semibold">{tr('errors.heading')}</h2>
      <p className="mt-2 text-sm text-muted">{error instanceof Error ? error.message : String(error)}</p>
      <Button className="mt-4" variant="secondary" onClick={onRetry ?? (() => { void qc.refetchQueries({ type: 'active', stale: true }); })}>{tr('actions.retry')}</Button>
    </div>
  );
}
