import { useMemo } from 'react';
import { Link, useSearchParams } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { FolderPlus, Info, Play, SlidersHorizontal } from 'lucide-react';
import { api, backdropUrl, itemHref, type Item, type ItemQuery, type Library, type RecentCard } from '@/lib/api';
import { resolveRows, rowHref, rowQuery, usePrefs, visibleRows, type ResolvedRow } from '@/lib/home';
import { runtime } from '@/lib/format';
import { Hero, MetaLine, TitleHeading } from '@/components/Hero';
import { cardClass, MediaRow, RowShell, type CardStyle } from '@/components/MediaRow';
import { PosterCard, ThumbCard, type CardOverrides } from '@/components/PosterCard';
import { EditHomeDialog } from '@/components/EditHome';
import { EmptyState, Page } from '@/components/Page';
import { useLibraries } from '@/components/Sidebar';
import { usePlayer } from '@/components/Player';
import { Button } from '@/components/ui/button';

function LibraryRow({ lib, style, showsOnly, seasonPosters }: { lib: Library; style: CardStyle; showsOnly: boolean; seasonPosters: boolean }) {
  const grouped = lib.kind === 'shows' && !showsOnly;
  const titles = useQuery({
    queryKey: ['items', { library_id: lib.id, sort: 'added', limit: 24 }],
    queryFn: () => api.items({ library_id: lib.id, kind: lib.kind === 'shows' ? 'series' : 'movie', sort: 'added', limit: 24 }),
    enabled: !grouped,
  });
  const cards = useQuery({ queryKey: ['items', 'recent-shows', lib.id], queryFn: () => api.recentShows(lib.id), enabled: grouped });
  const title = `Recently Added in ${lib.name}`;
  const href = `/library/${lib.id}`;
  if (!grouped) return <MediaRow title={title} items={titles.data?.items ?? []} href={href} style={style} />;
  return (
    <RowShell title={title} href={href} empty={!cards.data?.length}>
      {cards.data?.map((c) => {
        const Card = style === 'thumb' ? ThumbCard : PosterCard;
        return <Card key={`${c.type}:${c.item.id}:${c.season_number ?? ''}`} item={c.item} overrides={recentOverrides(c, style, seasonPosters)} className={cardClass(style)} />;
      })}
    </RowShell>
  );
}

/** How a Recently Added card reads: a season shows its own poster and how many episodes are new. */
function recentOverrides(c: RecentCard, style: CardStyle, seasonPosters: boolean): CardOverrides | undefined {
  const n = c.episodes;
  switch (c.type) {
    case 'series':
      return { subtitle: n > 1 ? `${n} new episodes` : '1 new episode' };
    case 'season': {
      const name = c.season_number === 0 ? 'Specials' : `Season ${c.season_number}`;
      return {
        image: style === 'poster' && seasonPosters ? (c.season_poster ?? c.item.images?.primary) : undefined,
        subtitle: `${name} · ${n} new episodes`,
        noBadge: true,
      };
    }
    default:
      return seasonPosters && style === 'poster' && c.item.images?.season_primary ? { image: c.item.images.season_primary } : undefined;
  }
}

function QueryRow({ title, query, href, style }: { title: string; query: ItemQuery; href?: string; style: CardStyle }) {
  const { data } = useQuery({ queryKey: ['items', query], queryFn: () => api.items(query) });
  return <MediaRow title={title} items={data?.items ?? []} href={href} style={style} />;
}

function HomeRowView({ row, withUpNext }: { row: ResolvedRow; withUpNext: boolean }) {
  const continueWatching = useQuery({
    queryKey: ['continue', withUpNext],
    queryFn: () => api.continueWatching(withUpNext),
    enabled: row.type === 'continue',
  });
  const nextUp = useQuery({ queryKey: ['next-up'], queryFn: api.nextUp, enabled: row.type === 'next-up' });
  switch (row.type) {
    case 'continue':
      return <MediaRow title={row.title} items={continueWatching.data ?? []} style={row.style} seasonPosters={row.posters === 'season'} play />;
    case 'next-up':
      return <MediaRow title={row.title} items={nextUp.data ?? []} style={row.style} seasonPosters={row.posters === 'season'} play />;
    case 'recent':
      return <LibraryRow lib={row.library} style={row.style} showsOnly={row.showsOnly} seasonPosters={row.posters === 'season'} />;
    case 'seasonal':
      return row.season ? <QueryRow title={row.season.title} query={{ ...row.season.query, limit: 24 }} style={row.style} /> : null;
    case 'custom': {
      const f = row.filter;
      // The library page can show the same list unless the row uses filters only rows have.
      const linkable = !f.added_within && !f.min_runtime && !f.max_runtime && !f.min_rating;
      return <QueryRow title={row.title} query={rowQuery(f, row.library)} href={linkable ? rowHref(f) : undefined} style={row.style} />;
    }
  }
}

export function Home() {
  const { data: libraries, isLoading } = useLibraries();
  const prefs = usePrefs();
  // ?edit=1 opens Edit Home, so the profile menu can link to it from anywhere.
  const [params, setParams] = useSearchParams();
  const editing = params.get('edit') === '1';
  const setEditing = (open: boolean) => setParams(open ? { edit: '1' } : {}, { replace: true });
  const split = !!prefs.data?.split_up_next;
  const rows = useMemo(() => (libraries && prefs.data ? resolveRows(prefs.data.home, libraries) : []), [libraries, prefs.data]);
  // The featured item: the most recently added thing with a backdrop.
  const featuredQuery = useQuery({
    queryKey: ['featured', libraries?.map((l) => l.id)],
    enabled: !!libraries?.length,
    queryFn: async () => {
      const lists = await Promise.all(
        libraries!.map((l) => api.items({ library_id: l.id, kind: l.kind === 'shows' ? 'series' : 'movie', sort: 'added', limit: 12 })),
      );
      const all = lists.flatMap((l) => l.items).filter((i) => i.images?.backdrop);
      all.sort((a, b) => b.added_at - a.added_at);
      return all[0] ?? null;
    },
  });

  if (isLoading) return null;
  if (!libraries?.length) {
    return (
      <Page>
        <EmptyState icon={<FolderPlus />} title="Welcome to Arc">
          <p>Add a folder of movies or TV shows to get started. Arc shows you what it finds before adding anything.</p>
          <Link to="/libraries?add=1" className="mt-5 inline-block">
            <Button variant="primary">
              <FolderPlus /> Add a Library
            </Button>
          </Link>
        </EmptyState>
      </Page>
    );
  }

  return (
    <Page className="relative pb-16">
      {prefs.data && (
        <Button
          size="sm"
          onClick={() => setEditing(true)}
          className="absolute right-6 top-5 z-20 bg-black/30 text-white ring-1 ring-white/15 backdrop-blur-md hover:bg-black/45 md:right-10"
        >
          <SlidersHorizontal /> Edit Home
        </Button>
      )}
      {featuredQuery.data ? <Featured item={featuredQuery.data} /> : <div className="h-10" />}
      <div className="relative z-10 -mt-6 space-y-10">
        {visibleRows(rows, split)
          .filter((r) => !r.hidden)
          .map((r) => (
            <HomeRowView key={r.id} row={r} withUpNext={!split} />
          ))}
      </div>
      {prefs.data && (
        <div className="mt-12 flex justify-center">
          <Button variant="ghost" size="sm" onClick={() => setEditing(true)}>
            <SlidersHorizontal /> Edit Home
          </Button>
        </div>
      )}
      <EditHomeDialog rows={rows} libraries={libraries} open={editing} onOpenChange={setEditing} />
    </Page>
  );
}

function Featured({ item }: { item: Item }) {
  const play = usePlayer();
  const canPlay = item.kind === 'movie' && item.stream_url;
  const meta = useMemo(() => [item.year, runtime(item.runtime_min), item.kind === 'series' && 'TV Series'], [item]);
  return (
    <Hero backdrop={backdropUrl(item)} className="h-[62vh] min-h-[420px] max-h-[720px]">
      <div className="flex h-full max-w-2xl flex-col justify-end px-6 pb-16 md:px-10">
        <p className="mb-3 text-xs font-semibold uppercase tracking-[0.18em] text-accent">Recently Added</p>
        <TitleHeading item={item} />
        <div className="mt-3">
          <MetaLine parts={meta} />
        </div>
        {item.overview && <p className="mt-4 line-clamp-3 max-w-xl text-[15px] leading-relaxed text-foreground/80">{item.overview}</p>}
        <div className="mt-6 flex gap-3">
          {canPlay && (
            <Button variant="primary" size="lg" onClick={() => play({ title: item.title, src: item.stream_url!, itemId: item.id, startMS: item.user_data?.position_ms })}>
              <Play className="fill-current" /> Play
            </Button>
          )}
          <Link to={itemHref(item)}>
            <Button variant="secondary" size="lg">
              <Info /> {item.kind === 'series' ? 'Episodes' : 'More Info'}
            </Button>
          </Link>
        </div>
      </div>
    </Hero>
  );
}
