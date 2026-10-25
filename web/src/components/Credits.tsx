import { Fragment, useRef } from 'react';
import { Link } from 'react-router';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import type { Credit, Item } from '@/lib/api';
import { cn } from '@/lib/utils';
import * as fmt from '@/lib/format';
import { Scores } from '@/components/Scores';

/** "Directed by …", "Written by …", "Created by …" and the studio or network. */
export function CrewLine({ item }: { item: Item }) {
  const credits = item.credits ?? [];
  const people = (role: Credit['role']) => credits.filter((c) => c.role === role).map((c) => ({ name: c.name, id: c.person_id }));
  const lines: [string, { name: string; id?: number }[]][] = [
    ['Created by', people('creator')],
    ['Directed by', people('director')],
    ['Written by', people('writer').slice(0, 3)],
    [item.kind === 'series' ? 'Network' : 'Studio', item.studio ? item.studio.split(', ').slice(0, 2).map((name) => ({ name })) : []],
    ['Language', item.original_language ? [{ name: fmt.language(item.original_language) }] : []],
    ['Status', showStatus(item) ? [{ name: showStatus(item) }] : []],
  ];
  const shown = lines.filter(([, n]) => n.length);
  if (!shown.length) return null;
  return (
    <dl className="mt-5 grid max-w-2xl grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
      {shown.map(([label, n]) => (
        <div key={label} className="contents">
          <dt className="text-subtle">{label}</dt>
          <dd className="text-foreground/85">
            {n.map((p, i) => (
              <Fragment key={p.name}>
                {i > 0 && ', '}
                {p.id ? (
                  <Link to={`/person/${p.id}`} className="hover:underline">
                    {p.name}
                  </Link>
                ) : (
                  p.name
                )}
              </Fragment>
            ))}
          </dd>
        </div>
      ))}
    </dl>
  );
}

/** "Ended in 2003", "Continuing · next episode 24 Oct 2026", and so on ("" if unknown). */
export function showStatus(item: Item): string {
  const year = item.last_aired?.slice(0, 4);
  const today = new Date().toISOString().slice(0, 10);
  switch (item.status) {
    case 'ended':
      return year ? `Ended in ${year}` : 'Ended';
    case 'cancelled':
      return year ? `Cancelled in ${year}` : 'Cancelled';
    case 'upcoming':
      return item.next_aired && item.next_aired >= today ? `Premieres ${fmt.date(item.next_aired)}` : 'Coming soon';
    case 'continuing':
      return item.next_aired && item.next_aired >= today ? `Continuing · next episode ${fmt.date(item.next_aired)}` : 'Continuing';
  }
  return '';
}

/** A scrolling row of the cast, with photos. */
export function CastRow({ credits, title = 'Cast' }: { credits?: Credit[]; title?: string }) {
  const scroller = useRef<HTMLDivElement>(null);
  const cast = (credits ?? []).filter((c) => c.role === 'actor');
  if (!cast.length) return null;
  const scroll = (dir: 1 | -1) => scroller.current?.scrollBy({ left: dir * scroller.current.clientWidth * 0.8, behavior: 'smooth' });
  return (
    <section className="group/row relative mt-6">
      <h2 className="mb-4 px-6 text-lg font-semibold md:px-10">{title}</h2>
      <div ref={scroller} className="scrollbar-none flex gap-5 overflow-x-auto px-6 pb-2 md:px-10">
        {cast.map((c) => (
          <Link key={c.person_id} to={`/person/${c.person_id}`} className="group/person w-24 shrink-0 text-center outline-none">
            <div className="mx-auto size-24 overflow-hidden rounded-full bg-surface-2 ring-1 ring-border transition-transform duration-300 group-hover/person:scale-105 group-focus-visible/person:ring-2 group-focus-visible/person:ring-accent">
              {c.image ? (
                <img src={c.image} alt="" loading="lazy" className="size-full object-cover" />
              ) : (
                <span className="flex size-full items-center justify-center text-2xl font-semibold text-subtle">{c.name.charAt(0)}</span>
              )}
            </div>
            <p className="mt-2 line-clamp-2 text-[13px] font-medium leading-tight">{c.name}</p>
            {c.character && <p className="mt-0.5 line-clamp-2 text-xs leading-tight text-subtle">{c.character}</p>}
            {!!c.episode_count && <p className="mt-0.5 text-xs text-subtle">{fmt.plural(c.episode_count, 'episode')}</p>}
          </Link>
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
    rating: item.imdb_rating || item.rotten_tomatoes != null || item.metacritic != null || item.rating ? <Scores item={item} /> : undefined,
    contentRating: item.content_rating,
    genres: (item.genres ?? []).slice(0, 3).join(', '),
  };
}
