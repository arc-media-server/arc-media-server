import { useState, type ReactNode } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { cn } from '@/lib/utils';
import type { Item } from '@/lib/api';

/**
 * Full-bleed backdrop with soft gradients that blend into the page, Apple TV style.
 * Content sits bottom-left over the artwork.
 */
export function Hero({ backdrop, children, className }: { backdrop?: string; children: ReactNode; className?: string }) {
  return (
    <div className={cn('relative isolate overflow-hidden', className)}>
      {backdrop ? (
        // Keyed by the picture, so a new one fades in over the last (the home page's rotating banner).
        <AnimatePresence initial={true}>
          <motion.img
            key={backdrop}
            src={backdrop}
            alt=""
            draggable={false}
            initial={{ opacity: 0, scale: 1.04 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, transition: { duration: 1 } }}
            transition={{ duration: 1.2, ease: [0.22, 1, 0.36, 1] }}
            className="absolute inset-0 -z-10 size-full object-cover object-[center_20%]"
          />
        </AnimatePresence>
      ) : (
        <div className="absolute inset-0 -z-10 bg-gradient-to-br from-accent-strong/30 via-surface to-background" />
      )}
      <div className="absolute inset-0 -z-10 bg-gradient-to-t from-background via-background/60 to-transparent" />
      <div className="absolute inset-0 -z-10 bg-gradient-to-r from-background/90 via-background/30 to-transparent" />
      {children}
    </div>
  );
}

/** The title's clear logo when it has one, otherwise its name in large type. */
export function TitleHeading({ item }: { item: Item }) {
  const logo = item.images?.logo;
  const [failed, setFailed] = useState<string>();
  if (!logo || failed === logo) return <h1 className="text-4xl font-bold leading-[1.05] md:text-6xl">{item.title}</h1>;
  return (
    <h1>
      <img
        src={logo}
        alt={item.title}
        draggable={false}
        onError={() => setFailed(logo)}
        className="max-h-24 w-auto max-w-[min(100%,26rem)] object-contain object-left drop-shadow-[0_2px_16px_rgb(0_0_0/0.45)] md:max-h-36"
      />
    </h1>
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
