import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import { cn } from '@/lib/utils';
import { backdropUrl, posterUrl, type Item } from '@/lib/api';
import { Artwork } from './Artwork';

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
export function TitleHeading({ item, detail = false }: { item: Item; detail?: boolean }) {
  const logo = item.images?.logo;
  const [failed, setFailed] = useState<string>();
  if (!logo || failed === logo) return <h1 className={cn('break-words font-bold leading-[1.05]', detail ? 'text-3xl sm:text-4xl xl:text-5xl' : 'text-4xl md:text-6xl')}>{item.title}</h1>;
  return (
    <h1>
      <img
        src={logo}
        alt={item.title}
        draggable={false}
        onError={() => setFailed(logo)}
        className={cn('w-auto max-w-[min(100%,26rem)] object-contain object-left drop-shadow-[0_2px_16px_rgb(0_0_0/0.45)]', detail ? 'max-h-20 md:max-h-28' : 'max-h-24 md:max-h-36')}
      />
    </h1>
  );
}

/** Shared title-page proportions; home videos retain their landscape artwork. */
export function TitleHero({ item, landscape = false, children }: { item: Item; landscape?: boolean; children: ReactNode }) {
  return (
    <Hero backdrop={backdropUrl(item)}>
      <div className={cn('grid items-start gap-8 px-6 pb-10 pt-20 md:px-10 md:pt-16 lg:pb-12 xl:gap-10', landscape ? 'lg:grid-cols-[320px_minmax(0,1fr)] xl:grid-cols-[384px_minmax(0,1fr)]' : 'lg:grid-cols-[240px_minmax(0,1fr)] xl:grid-cols-[256px_minmax(0,1fr)]')}>
        <Artwork
          src={posterUrl(item)} alt={item.title} fallbackTitle={item.title}
          icon={item.kind === 'series' ? 'tv' : 'film'} eager kind={landscape ? 'still' : 'poster'}
          className="hidden w-full rounded-2xl shadow-2xl shadow-black/40 ring-1 ring-white/10 lg:block"
        />
        <div className="min-w-0 w-full max-w-6xl">{children}</div>
      </div>
    </Hero>
  );
}

/** Keep long descriptions readable without moving the primary actions. */
export function Synopsis({ text }: { text: string }) {
  const id = useId();
  const paragraph = useRef<HTMLParagraphElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflows, setOverflows] = useState(false);
  useEffect(() => {
    const el = paragraph.current;
    if (!el) return;
    let active = true;
    const measure = () => {
      if (active) setOverflows(el.scrollHeight > parseFloat(getComputedStyle(el).lineHeight) * 4 + 1);
    };
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    measure();
    void document.fonts.ready.then(measure);
    return () => { active = false; observer.disconnect(); };
  }, [text, expanded]);
  return (
    <div className="max-w-[75ch]">
      <p ref={paragraph} id={id} className={cn('text-[15px] leading-relaxed text-foreground/85', !expanded && 'line-clamp-4')}>{text}</p>
      {overflows && <button type="button" aria-controls={id} aria-expanded={expanded} onClick={() => setExpanded(v => !v)} className="mt-2 rounded text-sm font-medium text-foreground underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent">{expanded ? 'Show less' : 'Show more'}</button>}
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
