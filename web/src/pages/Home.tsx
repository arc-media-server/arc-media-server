import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { useEffect, useMemo, useState, type ComponentProps } from 'react';
import { motion } from 'motion/react';
import { cn } from '@/lib/utils';
import { Link, useSearchParams } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { FolderPlus, Info, Play, SlidersHorizontal } from 'lucide-react';
import { api, backdropUrl, itemHref, type Item, type ItemQuery, type Library, type RecentCard } from '@/lib/api';
import { navigationLibraries } from '@/lib/library-navigation';
import { resolveRows, rowTitle, rowHref, rowQuery, usePrefs, visibleRows, type ResolvedRow } from '@/lib/home';
import { runtime } from '@/lib/format';
import { Hero, MetaLine, TitleHeading } from '@/components/Hero';
import { cardClass, MediaRow as SharedMediaRow, RowShell, type CardStyle } from '@/components/MediaRow';
import { PosterCard, ThumbCard, type CardOverrides } from '@/components/PosterCard';
import { useRemoveFromContinue } from '@/components/CardMenu';
import { HomeActionsProvider, HomeRowActions } from '@/components/HomeRowActions';
import { EditHomeDialog } from '@/components/EditHome';
import { EmptyState, ErrorState, ErrorNotice, Page } from '@/components/Page';
import { useLibraries } from '@/components/Sidebar';
import { usePlayer } from '@/components/Player';
import { useIsAdmin } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { useWatchRooms, WatchRoomCard } from '@/components/WatchTogether';

function MediaRow(props: ComponentProps<typeof SharedMediaRow>) {
  return <HomeRowActions items={props.items} title={props.title}>{cardProps => <SharedMediaRow {...props} cardProps={cardProps} />}</HomeRowActions>;
}

function LibraryRow({ lib, style, showsOnly, seasonPosters }: { lib: Library; style: CardStyle; showsOnly: boolean; seasonPosters: boolean }) {
  const { t } = useTranslation('home');
  const grouped = lib.kind === 'shows' && !showsOnly;
  const titles = useQuery({
    queryKey: ['items', { library_id: lib.id, sort: 'added', limit: 24 }],
    queryFn: () => api.items({ library_id: lib.id, kind: lib.kind === 'shows' ? 'series' : 'movie', sort: 'added', limit: 24 }),
    enabled: !grouped,
  });
  const cards = useQuery({ queryKey: ['items', 'recent-shows', lib.id], queryFn: () => api.recentShows(lib.id), enabled: grouped });
  const title = t('rows.recent', { name: lib.name });
  const href = `/library/${lib.id}?sort=added`;
  if (!grouped) return <MediaRow title={title} items={titles.data?.items ?? []} loading={titles.isPending} error={titles.error} onRetry={() => void titles.refetch()} href={href} style={style} />;
  return (
    <HomeRowActions items={cards.data?.map(c => c.item) ?? []} title={title}>{cardProps => <RowShell title={title} href={href} empty={!cards.data?.length} loading={cards.isPending} error={cards.error} onRetry={() => void cards.refetch()}>
      {cards.data?.map((c) => {
        const key = `${c.type}:${c.item.id}:${c.season_number ?? ''}:${c.added_at}`;
        const overrides = recentOverrides(c, style, seasonPosters);
        if (style === 'poster') return <PosterCard {...cardProps(c.item)} key={key} item={c.item} overrides={overrides} className={cardClass(style)} />;
        return <ThumbCard {...cardProps(c.item)} key={key} item={c.item} overrides={overrides} className={cardClass(style)} arc={style === 'arc'} />;
      })}
    </RowShell>}</HomeRowActions>
  );
}

/** How a Recently Added card reads: a season shows its own poster and how many episodes are new. */
function recentOverrides(c: RecentCard, style: CardStyle, seasonPosters: boolean): CardOverrides | undefined {
  const n = c.episodes;
  switch (c.type) {
    case 'series':
      return { subtitle: i18n.t('home:rows.newEpisodes', { count: Math.max(1, n) }) };
    case 'season': {
      const name = c.season_number === 0 ? i18n.t('home:rows.specials') : i18n.t('home:rows.season', { season: c.season_number });
      return {
        image: style === 'poster' && seasonPosters ? (c.season_poster ?? c.item.images?.primary) : undefined,
        subtitle: i18n.t('home:rows.seasonEpisodes', { season: name, count: n }),
        noBadge: true,
      };
    }
    default:
      return seasonPosters && style === 'poster' && c.item.images?.season_primary ? { image: c.item.images.season_primary } : undefined;
  }
}

/** The profile's watchlist (hidden while it's empty). */
function WatchlistRow({ style }: { style: CardStyle }) {
  const { t } = useTranslation('home');
  const result = useQuery({ queryKey: ['watchlist'], queryFn: api.watchlist });
  const { data } = result;
  return <MediaRow title={t('common:navigation.watchlist')} items={data ?? []} loading={result.isPending} error={result.error} onRetry={() => void result.refetch()} href="/watchlist" style={style} />;
}

/** A collection on the home page, as it is now. */
function CollectionRow({ id, fallback, style }: { id: string; fallback: string; style: CardStyle }) {
  const result = useQuery({ queryKey: ['collection', id], queryFn: () => api.collection(id), retry: false });
  const { data } = result;
  return <MediaRow title={data?.collection.title ?? fallback} items={data?.items ?? []} loading={result.isPending} error={result.error} onRetry={() => void result.refetch()} href={`/collection/${id}`} style={style} />;
}

/** One of the profile's playlists on the home page, in its order. */
function PlaylistRow({ id, fallback, style, seasonPosters }: { id: string; fallback: string; style: CardStyle; seasonPosters: boolean }) {
  const result = useQuery({ queryKey: ['playlist', id], queryFn: () => api.playlist(id), retry: false });
  const { data } = result;
  return (
    <MediaRow
      title={data?.playlist.title ?? fallback}
      items={data?.entries.map((e) => e.item) ?? []}
      loading={result.isPending} error={result.error} onRetry={() => void result.refetch()}
      href={`/playlist/${id}`}
      style={style}
      seasonPosters={seasonPosters}
    />
  );
}

function QueryRow({ title, query, href, style }: { title: string; query: ItemQuery; href?: string; style: CardStyle }) {
  const result = useQuery({ queryKey: ['items', query], queryFn: () => api.items(query) });
  const { data } = result;
  return <MediaRow title={title} items={data?.items ?? []} loading={result.isPending} error={result.error} onRetry={() => void result.refetch()} href={href} style={style} />;
}

/** Watch Together rooms this profile made, was invited to or is in. Shown only when there are some. */
function WatchTogetherRow() {
  const { t } = useTranslation('player');
  const result = useWatchRooms();
  const list = result.data ?? [];
  if (!list.length) return null;
  return (
    <RowShell title={t('player:together.rowTitle')}>
      {list.map((room) => <WatchRoomCard key={room.id} room={room} />)}
    </RowShell>
  );
}

function HomeRowView({ row, withUpNext }: { row: ResolvedRow; withUpNext: boolean }) {
  const remove = useRemoveFromContinue();
  const continueWatching = useQuery({
    queryKey: ['continue', withUpNext],
    queryFn: () => api.continueWatching(withUpNext),
    enabled: row.type === 'continue',
  });
  const nextUp = useQuery({ queryKey: ['next-up'], queryFn: api.nextUp, enabled: row.type === 'next-up' });
  switch (row.type) {
    case 'continue':
      return <MediaRow title={rowTitle(row)} items={continueWatching.data ?? []} loading={continueWatching.isPending} error={continueWatching.error || remove.error} onRetry={() => { if (remove.error && remove.variables) remove.mutate(remove.variables); else void continueWatching.refetch(); }} style={row.style} seasonPosters={row.posters === 'season'} play onRemove={(i) => remove.mutate(i)} />;
    case 'next-up':
      return <MediaRow title={rowTitle(row)} items={nextUp.data ?? []} loading={nextUp.isPending} error={nextUp.error || remove.error} onRetry={() => { if (remove.error && remove.variables) remove.mutate(remove.variables); else void nextUp.refetch(); }} style={row.style} seasonPosters={row.posters === 'season'} play onRemove={(i) => remove.mutate(i)} />;
    case 'recent':
      return <LibraryRow lib={row.library} style={row.style} showsOnly={row.showsOnly} seasonPosters={row.posters === 'season'} />;
    case 'seasonal':
      return row.season ? <QueryRow title={row.season.title} query={{ ...row.season.query, limit: 24 }} style={row.style} /> : null;
    case 'watchlist':
      return <WatchlistRow style={row.style} />;
    case 'collection':
      return <CollectionRow id={row.ref} fallback={rowTitle(row)} style={row.style} />;
    case 'playlist':
      return <PlaylistRow id={row.ref} fallback={rowTitle(row)} style={row.style} seasonPosters={row.posters === 'season'} />;
    case 'custom': {
      const f = row.filter;
      // The library page can show the same list unless the row uses filters only rows have.
      return <QueryRow title={rowTitle(row)} query={rowQuery(f, row.library)} href={rowHref(f)} style={row.style} />;
    }
  }
}

export function Home() {
  const { t } = useTranslation('home');
  const libraryQuery = useLibraries();
  const { data: libraries, isLoading } = libraryQuery;
  const admin = useIsAdmin();
  const prefs = usePrefs();
  // ?edit=1 opens Edit Home, so the profile menu can link to it from anywhere.
  const [params, setParams] = useSearchParams();
  const editing = params.get('edit') === '1';
  const setEditing = (open: boolean) => setParams(open ? { edit: '1' } : {}, { replace: true });
  const split = !!prefs.data?.split_up_next;
  const showHero = !!prefs.data && prefs.data.show_home_hero !== false;
  const rows = useMemo(() => (libraries && prefs.data ? resolveRows(prefs.data.home, navigationLibraries(libraries, prefs.data.library_order), prefs.data.watchlist_position) : []), [libraries, prefs.data, t]);
  // The banner: the most recently added titles with a backdrop, taking turns.
  const featuredQuery = useQuery({
    queryKey: ['featured', libraries?.map((l) => l.id)],
    enabled: showHero && !!libraries?.length,
    queryFn: async () => {
      // Home videos stay out of it: they're personal, and rarely have artwork worth a banner.
      const lists = await Promise.all(
        libraries!.filter((l) => l.kind !== 'videos').map((l) => api.items({ library_id: l.id, kind: l.kind === 'shows' ? 'series' : 'movie', sort: 'added', limit: 12 })),
      );
      const all = lists.flatMap((l) => l.items).filter((i) => i.images?.backdrop);
      all.sort((a, b) => b.added_at - a.added_at);
      return all.slice(0, featuredCount);
    },
  });

  const hasFeatured = showHero && !!featuredQuery.data?.length;

  if (isLoading) return <div className="m-6 h-64 animate-pulse rounded-2xl bg-surface" />;
  if (libraryQuery.error && !libraries) return <ErrorState error={libraryQuery.error} onRetry={() => void libraryQuery.refetch()} />;
  if (prefs.error && !prefs.data) return <ErrorState error={prefs.error} onRetry={() => void prefs.refetch()} />;
  if (!libraries?.length) {
    return (
      <Page>
        <EmptyState icon={<FolderPlus />} title={t('empty.heading')}>
          <p>{admin ? t('empty.admin') : t('empty.viewer')}</p>
          {admin && <Link to="/libraries?add=1" className="mt-5 inline-block">
            <Button variant="primary">
              <FolderPlus />{' '}{t('empty.addLibrary')}</Button>
          </Link>}
        </EmptyState>
      </Page>
    );
  }

  return (
    <HomeActionsProvider><Page className="relative pb-36">
      {!!libraryQuery.error && <ErrorNotice error={libraryQuery.error} onRetry={() => void libraryQuery.refetch()} />}
      {!!prefs.error && <ErrorNotice error={prefs.error} onRetry={() => void prefs.refetch()} />}
      {prefs.data && (
        <div className={hasFeatured ? 'absolute right-6 top-5 z-20 md:right-10' : 'flex justify-end px-6 py-5 md:px-10'}>
          <Button size="sm" onClick={() => setEditing(true)} className={hasFeatured ? 'bg-black/30 text-white ring-1 ring-white/15 backdrop-blur-md hover:bg-black/45' : undefined}>
            <SlidersHorizontal />{' '}{t('editor.title')}</Button>
        </div>
      )}
      {hasFeatured && <section aria-label={t('featured.label')}><Featured items={featuredQuery.data!} /></section>}
      <div className={cn('relative z-10 space-y-10', hasFeatured && '-mt-6')}>
        {prefs.isPending && <div className="mx-6 h-48 animate-pulse rounded-xl bg-surface" />}
        {showHero && !!featuredQuery.error && <div className="px-6 md:px-10"><ErrorNotice error={featuredQuery.error} onRetry={() => void featuredQuery.refetch()} /></div>}
        <WatchTogetherRow />
        {visibleRows(rows, split)
          .filter((r) => !r.hidden)
          .map((r) => (
            <HomeRowView key={r.id} row={r} withUpNext={!split} />
          ))}
      </div>
      {prefs.data && (
        <div className="mt-12 flex justify-center">
          <Button variant="ghost" size="sm" onClick={() => setEditing(true)}>
            <SlidersHorizontal />{' '}{t('editor.title')}</Button>
        </div>
      )}
      <EditHomeDialog rows={rows} libraries={navigationLibraries(libraries, prefs.data?.library_order)} watchlistAt={prefs.data?.watchlist_position} open={editing} onOpenChange={setEditing} />
    </Page></HomeActionsProvider>
  );
}

// How many titles the banner takes turns with, and how long each shows.
const featuredCount = 6;
const featuredMS = 8000;

function Featured({ items }: { items: Item[] }) {
  const { t } = useTranslation('home');
  const [index, setIndex] = useState(0);
  const [paused, setPaused] = useState(false);
  const item = items[index % items.length];
  // Move on every few seconds, unless the pointer is over it or the device asks for less motion.
  useEffect(() => {
    if (items.length < 2 || paused || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const timer = window.setTimeout(() => setIndex((i) => (i + 1) % items.length), featuredMS);
    return () => window.clearTimeout(timer);
  }, [index, items.length, paused]);
  return (
    <div onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)} className="relative">
      <FeaturedItem item={item} />
      {items.length > 1 && (
        <div className="absolute bottom-16 right-6 z-10 flex gap-1.5 md:right-10" role="tablist" aria-label={t('featured.label')}>
          {items.map((it, i) => (
            <button
              key={it.id}
              role="tab"
              aria-selected={i === index % items.length}
              aria-label={it.title}
              title={it.title}
              onClick={() => setIndex(i)}
              className={cn('h-1.5 rounded-full transition-all duration-300', i === index % items.length ? 'w-6 bg-foreground' : 'w-1.5 bg-foreground/35 hover:bg-foreground/60')}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function FeaturedItem({ item }: { item: Item }) {
  const { t } = useTranslation('home');
  const play = usePlayer();
  const canPlay = item.kind === 'movie' && item.stream_url;
  const meta = useMemo(() => [item.year, runtime(item.runtime_min), item.kind === 'series' && t('featured.tvSeries')], [item, t]);
  return (
    <Hero backdrop={backdropUrl(item)} className="h-[62vh] min-h-[420px] max-h-[720px]">
      <motion.div
        key={item.id}
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
        className="flex h-full max-w-2xl flex-col justify-end px-6 pb-16 md:px-10"
      >
        <p className="mb-3 text-xs font-semibold uppercase tracking-[0.18em] text-accent">{t('featured.recent')}</p>
        <TitleHeading item={item} />
        <div className="mt-3">
          <MetaLine parts={meta} />
        </div>
        {item.overview && <p className="mt-4 line-clamp-3 max-w-xl text-[15px] leading-relaxed text-foreground/80">{item.overview}</p>}
        <div className="mt-6 flex gap-3">
          {canPlay && (
            <Button variant="primary" size="lg" onClick={() => play({ title: item.title, src: item.stream_url!, itemId: item.id, startMS: item.user_data?.position_ms })}>
              <Play className="fill-current" />{' '}{t('common:actions.play')}</Button>
          )}
          <Link to={itemHref(item)}>
            <Button variant="secondary" size="lg">
              <Info /> {item.kind === 'series' ? t('featured.episodes') : t('featured.moreInfo')}
            </Button>
          </Link>
        </div>
      </motion.div>
    </Hero>
  );
}
