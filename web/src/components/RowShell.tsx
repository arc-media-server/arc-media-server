import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Link } from 'react-router';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { ErrorNotice } from './Page';
import { Button } from './ui/button';
import { cn } from '@/lib/utils';

/** Shared mouse/keyboard navigation for posters, cast and extras. Native wheel/swipe stays intact. */
export function RowShell({ title, href, empty, children, loading, error, onRetry, className }: { title: string; href?: string; empty?: boolean; children: ReactNode; loading?: boolean; error?: unknown; onRetry?: () => void; className?: string }) {
  const scroller = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const helpId = useId();
  const [bounds, setBounds] = useState({ left: false, right: false });
  const shown = !(empty && !loading && !error);
  useEffect(() => {
    const el = scroller.current;
    if (!el) return;
    const measure = () => {
      const left = el.scrollLeft > 2;
      const right = el.scrollWidth - el.clientWidth - el.scrollLeft > 2;
      setBounds(old => old.left === left && old.right === right ? old : { left, right });
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    for (const child of el.children) observer.observe(child);
    el.addEventListener('scroll', measure, { passive: true });
    return () => { observer.disconnect(); el.removeEventListener('scroll', measure); };
  }, [children, shown, loading]);
  if (!shown) return null;

  const scroll = (direction: 1 | -1) => {
    const el = scroller.current;
    if (el) el.scrollBy({ left: direction * el.clientWidth * 0.85, behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
  };
  return (
    <section aria-labelledby={titleId} className={cn('relative', className)}>
      <div className="mb-3 flex items-center justify-between gap-4 px-6 md:px-10">
        <h2 id={titleId} className="min-w-0 text-lg font-semibold">
          {href ? <Link to={href} className="inline-flex items-center gap-1 hover:text-accent">{title}<ChevronRight className="size-4 shrink-0 text-subtle" /></Link> : title}
        </h2>
        {(bounds.left || bounds.right) && <div className="flex shrink-0 gap-1">
          <Button type="button" variant="secondary" size="icon" aria-label={`Scroll ${title} left`} disabled={!bounds.left} onClick={() => scroll(-1)} className="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"><ChevronLeft className="size-5" /></Button>
          <Button type="button" variant="secondary" size="icon" aria-label={`Scroll ${title} right`} disabled={!bounds.right} onClick={() => scroll(1)} className="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"><ChevronRight className="size-5" /></Button>
        </div>}
      </div>
      {!!error && <div className="mb-3 px-6 md:px-10"><ErrorNotice error={error} onRetry={onRetry} /></div>}
      <p id={helpId} className="sr-only">Use Left and Right arrows to browse this row, Home and End to jump to its ends, or Tab to choose a title.</p>
      <div ref={scroller} role="group" aria-label={`${title} cards`} aria-describedby={helpId} tabIndex={0}
        className="scrollbar-none flex snap-x snap-mandatory gap-5 overflow-x-auto scroll-px-6 px-6 pb-2 pt-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent md:scroll-px-10 md:px-10"
        onKeyDown={event => {
          if (event.target !== event.currentTarget || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
          if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); scroll(event.key === 'ArrowLeft' ? -1 : 1); }
          if (event.key === 'Home' || event.key === 'End') { event.preventDefault(); event.currentTarget.scrollTo({ left: event.key === 'Home' ? 0 : event.currentTarget.scrollWidth, behavior: 'instant' }); }
        }}
        onFocusCapture={event => {
          const el = event.currentTarget;
          const card = Array.from(el.children).find(child => child.contains(event.target));
          if (!card) return;
          const frame = el.getBoundingClientRect();
          const rect = card.getBoundingClientRect();
          const delta = rect.left < frame.left + 8 ? rect.left - frame.left - 8 : rect.right > frame.right - 8 ? rect.right - frame.right + 8 : 0;
          if (delta) el.scrollBy({ left: delta, behavior: 'instant' });
        }}>
        {loading && empty ? Array.from({ length: 4 }, (_, i) => <div key={i} className="h-48 w-32 shrink-0 animate-pulse rounded-xl bg-surface" />) : children}
      </div>
    </section>
  );
}
