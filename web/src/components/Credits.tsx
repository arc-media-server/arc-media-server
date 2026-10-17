import { useRef } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import type { Credit, Item } from '@/lib/api';
import { cn } from '@/lib/utils';

/** "Directed by …", "Written by …", "Created by …" and the studio or network. */
export function CrewLine({ item }: { item: Item }) {
  const credits = item.credits ?? [];
  const names = (role: Credit['role']) => credits.filter((c) => c.role === role).map((c) => c.name);
  const lines = [
    ['Created by', names('creator')],
    ['Directed by', names('director')],
    ['Written by', names('writer').slice(0, 3)],
    [item.kind === 'series' ? 'Network' : 'Studio', item.studio ? item.studio.split(', ').slice(0, 2) : []],
  ] as const;
  const shown = lines.filter(([, n]) => n.length);
  if (!shown.length) return null;
  return (
    <dl className="mt-5 grid max-w-2xl grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
      {shown.map(([label, n]) => (
        <div key={label} className="contents">
          <dt className="text-subtle">{label}</dt>
          <dd className="text-foreground/85">{n.join(', ')}</dd>
        </div>
      ))}
    </dl>
  );
}

/** A scrolling row of the cast, with photos. */
export function CastRow({ credits }: { credits?: Credit[] }) {
  const scroller = useRef<HTMLDivElement>(null);
  const cast = (credits ?? []).filter((c) => c.role === 'actor');
  if (!cast.length) return null;
  const scroll = (dir: 1 | -1) => scroller.current?.scrollBy({ left: dir * scroller.current.clientWidth * 0.8, behavior: 'smooth' });
  return (
    <section className="group/row relative mt-10">
      <h2 className="mb-4 px-6 text-lg font-semibold md:px-10">Cast</h2>
      <div ref={scroller} className="scrollbar-none flex gap-5 overflow-x-auto px-6 pb-2 md:px-10">
        {cast.map((c) => (
          <div key={c.person_id} className="w-24 shrink-0 text-center">
            <div className="mx-auto size-24 overflow-hidden rounded-full bg-surface-2 ring-1 ring-border">
              {c.image ? (
                <img src={c.image} alt="" loading="lazy" className="size-full object-cover" />
              ) : (
                <span className="flex size-full items-center justify-center text-2xl font-semibold text-subtle">{c.name.charAt(0)}</span>
              )}
            </div>
            <p className="mt-2 line-clamp-2 text-[13px] font-medium leading-tight">{c.name}</p>
            {c.character && <p className="mt-0.5 line-clamp-2 text-xs leading-tight text-subtle">{c.character}</p>}
          </div>
        ))}
      </div>
      {cast.length > 8 &&
        ([-1, 1] as const).map((dir) => (
          <button
            key={dir}
            type="button"
            aria-label={dir < 0 ? 'Scroll left' : 'Scroll right'}
            onClick={() => scroll(dir)}
            className={cn(
              'glass absolute top-[60px] hidden size-10 items-center justify-center rounded-full border border-border opacity-0 shadow-xl transition-opacity group-hover/row:opacity-100 md:flex',
              dir < 0 ? 'left-3' : 'right-3',
            )}
          >
            {dir < 0 ? <ChevronLeft className="size-5" /> : <ChevronRight className="size-5" />}
          </button>
        ))}
    </section>
  );
}

/** The rating, age rating and genres for the meta line under a title. */
export function detailParts(item: Item) {
  return {
    rating: item.rating ? `★ ${item.rating.toFixed(1)}` : undefined,
    contentRating: item.content_rating,
    genres: (item.genres ?? []).slice(0, 3).join(', '),
  };
}
