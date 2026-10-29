import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { Fragment } from 'react';
import { Link } from 'react-router';
import type { Credit, Item } from '@/lib/api';
import { RowShell } from '@/components/RowShell';
import * as fmt from '@/lib/format';
import { Scores } from '@/components/Scores';

/** "Directed by …", "Written by …", "Created by …" and the studio or network. */
export function CrewLine({ item }: { item: Item }) {
  const { t } = useTranslation('titles');
  const credits = item.credits ?? [];
  const people = (role: Credit['role']) => credits.filter((c) => c.role === role).map((c) => ({ name: c.name, id: c.person_id }));
  const lines: [string, { name: string; id?: number }[]][] = [
    [t('titles:credits.createdBy'), people('creator')],
    [t('titles:credits.directedBy'), people('director')],
    [t('titles:credits.writtenBy'), people('writer').slice(0, 3)],
    [item.kind === 'series' ? t('titles:credits.network') : t('titles:credits.studio'), item.studio ? item.studio.split(', ').slice(0, 2).map((name) => ({ name })) : []],
    [t('titles:credits.language'), item.original_language ? [{ name: fmt.language(item.original_language) }] : []],
    [t('titles:credits.status'), showStatus(item) ? [{ name: showStatus(item) }] : []],
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
      return year ? i18n.t('titles:credits.endedYear', { year }) : i18n.t('titles:credits.ended');
    case 'cancelled':
      return year ? i18n.t('titles:credits.cancelledYear', { year }) : i18n.t('titles:credits.cancelled');
    case 'upcoming':
      return item.next_aired && item.next_aired >= today ? i18n.t('titles:credits.premieres', { date: fmt.date(item.next_aired) }) : i18n.t('titles:credits.comingSoon');
    case 'continuing':
      return item.next_aired && item.next_aired >= today ? i18n.t('titles:credits.nextEpisode', { date: fmt.date(item.next_aired) }) : i18n.t('titles:credits.continuing');
  }
  return '';
}

/** A scrolling row of the cast, with photos. */
export function CastRow({ credits, title }: { credits?: Credit[]; title?: string }) {
  const { t } = useTranslation('titles');
  const cast = (credits ?? []).filter((c) => c.role === 'actor');
  if (!cast.length) return null;
  return (
    <RowShell title={title ?? t('titles:credits.cast')} className="mt-6">
        {cast.map((c) => (
          <Link key={c.person_id} to={`/person/${c.person_id}`} className="group/person w-24 shrink-0 snap-start text-center outline-none">
            <div className="mx-auto size-24 overflow-hidden rounded-full bg-surface-2 ring-1 ring-border transition-transform duration-300 group-hover/person:scale-105 group-focus-visible/person:ring-2 group-focus-visible/person:ring-accent">
              {c.image ? (
                <img src={c.image} alt="" loading="lazy" className="size-full object-cover" />
              ) : (
                <span className="flex size-full items-center justify-center text-2xl font-semibold text-subtle">{c.name.charAt(0)}</span>
              )}
            </div>
            <p className="mt-2 line-clamp-2 text-[13px] font-medium leading-tight">{c.name}</p>
            {c.character && <p className="mt-0.5 line-clamp-2 text-xs leading-tight text-subtle">{c.character}</p>}
            {!!c.episode_count && <p className="mt-0.5 text-xs text-subtle">{t('titles:credits.episodes', { count: c.episode_count })}</p>}
          </Link>
        ))}
    </RowShell>
  );
}

/** The rating, age rating and genres for the meta line under a title. */
export function detailParts(item: Item) {
  return {
    rating: item.rotten_tomatoes != null || item.metacritic != null || item.rating ? <Scores item={item} /> : undefined,
    contentRating: item.content_rating,
    genres: (item.genres ?? []).slice(0, 3).join(', '),
  };
}
