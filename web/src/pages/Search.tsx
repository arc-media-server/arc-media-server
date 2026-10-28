import { useTranslation } from 'react-i18next';
import type { ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { SearchX, Tag } from 'lucide-react';
import { api, type Person } from '@/lib/api';
import { countUnit } from '@/lib/browsing-labels';
import { CollectionCard } from '@/components/Collections';
import { ResumeCard } from '@/components/EpisodeCard';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';


export function SearchPage() {
  const { t } = useTranslation('search');
  const [params] = useSearchParams();
  const q = params.get('q')?.trim() ?? '';
  const results = useQuery({
    queryKey: ['search', q],
    enabled: q.length > 0,
    queryFn: () => api.search(q),
  });

  const d = results.data;
  const none = d && !d.movies.length && !d.shows.length && !d.episodes.length && !d.people.length && !d.collections.length && !d.genres.length;

  return (
    <Page className="pb-16">
      <PageHeader title={q ? t('search:resultsFor', { query: q }) : t('common:navigation.search')} subtitle={q ? undefined : t('search:description')} />
      {q && results.isFetching && <p role="status" className="mb-4 px-6 text-sm text-muted md:px-10">{t('search:searching')}</p>}
      <div className="space-y-12 px-6 md:px-10" aria-busy={!!q && results.isFetching}>
        {!q ? null : results.error ? (
          <ErrorState error={results.error} />
        ) : !d ? (
          <PosterGridSkeleton count={6} />
        ) : none ? (
          <EmptyState icon={<SearchX />} title={t('search:emptyTitle')}>
            {t('search:emptyDescription', { query: q })}
          </EmptyState>
        ) : (
          <>
            {d.movies.length > 0 && (
              <Section title={t('search:sections.movies')}>
                <PosterGrid items={d.movies} hover={{}} />
              </Section>
            )}
            {d.shows.length > 0 && (
              <Section title={t('search:sections.shows')}>
                <PosterGrid items={d.shows} hover={{}} />
              </Section>
            )}
            {d.people.length > 0 && (
              <Section title={t('search:sections.people')}>
                <div className="grid grid-cols-[repeat(auto-fill,minmax(112px,1fr))] gap-x-4 gap-y-6">
                  {d.people.map((p) => (
                    <PersonCard key={p.id} person={p} />
                  ))}
                </div>
              </Section>
            )}
            {d.collections.length > 0 && (
              <Section title={t('search:sections.collections')}>
                <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-8 pt-1.5 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
                  {d.collections.map((c) => (
                    <CollectionCard key={c.id} collection={c} unit="title" />
                  ))}
                </div>
              </Section>
            )}
            {d.episodes.length > 0 && (
              <Section title={t('search:sections.episodes')}>
                <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-5">
                  {d.episodes.map((e) => (
                    <ResumeCard key={e.id} item={e} />
                  ))}
                </div>
              </Section>
            )}
            {d.genres.length > 0 && (
              <Section title={t('search:sections.genres')}>
                <div className="flex flex-wrap gap-2">
                  {d.genres.map((g) => (
                    <Link
                      key={`${g.library_id}-${g.name}`}
                      to={`/library/${g.library_id}?genre=${encodeURIComponent(g.name)}`}
                      className="flex items-center gap-2 rounded-full bg-surface px-4 py-2 text-sm ring-1 ring-border transition-colors hover:bg-surface-2"
                    >
                      <Tag className="size-3.5 text-accent" /> {g.name}
                      <span className="text-subtle">
                        {g.library_name} · {g.count}
                      </span>
                    </Link>
                  ))}
                </div>
              </Section>
            )}
          </>
        )}
      </div>
    </Page>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section>
      <h2 className="mb-4 text-lg font-semibold">{title}</h2>
      {children}
    </section>
  );
}

export function PersonCard({ person: p }: { person: Person }) {
  const { t } = useTranslation('search');
  const knownFor: Record<string, string> = { actor: t('search:roles.actor'), director: t('search:roles.director'), writer: t('search:roles.writer'), creator: t('search:roles.creator') };
  return (
    <Link to={`/person/${p.id}`} className="group block text-center outline-none">
      <div className="mx-auto aspect-square w-full max-w-28 overflow-hidden rounded-full bg-surface-2 ring-1 ring-border transition-transform duration-300 group-hover:scale-105 group-focus-visible:ring-2 group-focus-visible:ring-accent">
        {p.image ? (
          <img src={p.image} alt="" loading="lazy" className="size-full object-cover" />
        ) : (
          <span className="flex size-full items-center justify-center text-3xl font-semibold text-subtle">{p.name.charAt(0)}</span>
        )}
      </div>
      <p className="mt-2 line-clamp-2 text-[13px] font-medium leading-tight">{p.name}</p>
      <p className="mt-0.5 text-xs text-subtle">{[p.known_for && knownFor[p.known_for], countUnit(p.count, 'title')].filter(Boolean).join(' · ')}</p>
    </Link>
  );
}
