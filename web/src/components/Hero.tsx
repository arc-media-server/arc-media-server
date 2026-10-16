import type { ReactNode } from 'react';
import { motion } from 'motion/react';
import { cn } from '@/lib/utils';

/**
 * Full-bleed backdrop with soft gradients that blend into the page, Apple TV style.
 * Content sits bottom-left over the artwork.
 */
export function Hero({ backdrop, children, className }: { backdrop?: string; children: ReactNode; className?: string }) {
  return (
    <div className={cn('relative isolate overflow-hidden', className)}>
      {backdrop ? (
        <motion.img
          src={backdrop}
          alt=""
          draggable={false}
          initial={{ opacity: 0, scale: 1.04 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 1.2, ease: [0.22, 1, 0.36, 1] }}
          className="absolute inset-0 -z-10 size-full object-cover object-[center_20%]"
        />
      ) : (
        <div className="absolute inset-0 -z-10 bg-gradient-to-br from-accent-strong/30 via-surface to-background" />
      )}
      <div className="absolute inset-0 -z-10 bg-gradient-to-t from-background via-background/60 to-transparent" />
      <div className="absolute inset-0 -z-10 bg-gradient-to-r from-background/90 via-background/30 to-transparent" />
      {children}
    </div>
  );
}

export function MetaLine({ parts }: { parts: (ReactNode | false | undefined | null | '')[] }) {
  const shown = parts.filter(Boolean);
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted">
      {shown.map((p, i) => (
        <span key={i} className="flex items-center gap-2">
          {i > 0 && <span className="text-subtle">·</span>}
          {p}
        </span>
      ))}
    </div>
  );
}

export function Badge({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span className={cn('rounded-md border border-foreground/25 px-1.5 py-px text-[11px] font-semibold tracking-wide text-foreground/80', className)}>
      {children}
    </span>
  );
}
