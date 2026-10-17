import { useRef } from 'react';
import { Link } from 'react-router';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import type { Item } from '@/lib/api';
import { ResumeCard } from './EpisodeCard';
import { PosterCard } from './PosterCard';

/**
 * A titled, horizontally scrolling row of posters (or, with `landscape`, cards to carry on
 * watching) with arrow buttons on hover.
 */
export function MediaRow({ title, items, href, landscape }: { title: string; items: Item[]; href?: string; landscape?: boolean }) {
  const scroller = useRef<HTMLDivElement>(null);
  if (items.length === 0) return null;

  const scroll = (dir: 1 | -1) => {
    const el = scroller.current;
    if (el) el.scrollBy({ left: dir * el.clientWidth * 0.85, behavior: 'smooth' });
  };

  return (
    <section className="group/row relative">
      <div className="mb-3 flex items-baseline justify-between px-6 md:px-10">
        <h2 className="text-lg font-semibold">
          {href ? (
            <Link to={href} className="inline-flex items-center gap-1 hover:text-accent">
              {title}
              <ChevronRight className="size-4 text-subtle" />
            </Link>
          ) : (
            title
          )}
        </h2>
      </div>
      <div className="relative">
        <div
          ref={scroller}
          className="scrollbar-none flex snap-x snap-mandatory gap-5 overflow-x-auto scroll-px-6 px-6 pb-2 pt-1 md:scroll-px-10 md:px-10"
        >
          {items.map((item) =>
            landscape ? (
              <ResumeCard key={item.id} item={item} className="w-[260px] shrink-0 snap-start sm:w-[300px]" />
            ) : (
              <PosterCard key={item.id} item={item} className="w-[132px] shrink-0 snap-start sm:w-[156px]" />
            ),
          )}
        </div>
        <RowArrow side="left" onClick={() => scroll(-1)} />
        <RowArrow side="right" onClick={() => scroll(1)} />
      </div>
    </section>
  );
}

function RowArrow({ side, onClick }: { side: 'left' | 'right'; onClick: () => void }) {
  const Icon = side === 'left' ? ChevronLeft : ChevronRight;
  return (
    <button
      type="button"
      aria-label={side === 'left' ? 'Scroll left' : 'Scroll right'}
      onClick={onClick}
      className={`glass absolute top-[38%] hidden size-10 -translate-y-1/2 items-center justify-center rounded-full border border-border opacity-0 shadow-xl transition-opacity duration-200 group-hover/row:opacity-100 md:flex ${side === 'left' ? 'left-3' : 'right-3'}`}
    >
      <Icon className="size-5" />
    </button>
  );
}
