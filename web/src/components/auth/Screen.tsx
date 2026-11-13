import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { motion } from 'motion/react';
import { ArcLogo } from '@/components/ArcLogo';

/** Full-screen backdrop for the sign-in, setup and profile screens. */
export function AuthScreen({ children, wide }: { children: ReactNode; wide?: boolean }) {
  const { t } = useTranslation('common');
  return (
    <div className="relative flex min-h-dvh items-center justify-center overflow-hidden px-4 py-12">
      <div className="pointer-events-none absolute -top-1/3 left-1/2 size-[900px] -translate-x-1/2 rounded-full bg-accent-strong/15 blur-3xl" />
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }}
        className={wide ? 'relative w-full max-w-3xl' : 'relative w-full max-w-md'}
      >
        <div className="mb-8 flex items-center justify-center gap-2.5">
          <ArcLogo className="h-12 w-auto" label={t('common:productName')} />
        </div>
        {children}
      </motion.div>
    </div>
  );
}

export function Card({ children }: { children: ReactNode }) {
  return <div className="glass rounded-3xl border border-border p-7 shadow-2xl shadow-black/30">{children}</div>;
}

export const fieldClass =
  'h-11 w-full rounded-xl bg-surface-2/80 px-3.5 text-[15px] ring-1 ring-border outline-none transition-shadow placeholder:text-subtle focus:ring-2 focus:ring-accent';

export function Label({ children }: { children: ReactNode }) {
  return <span className="mb-1.5 block text-[13px] font-medium text-muted">{children}</span>;
}
