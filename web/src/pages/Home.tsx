import { useMemo, useState } from 'react';
import { Link } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { FolderPlus, Info, Play, SlidersHorizontal } from 'lucide-react';
import { api, backdropUrl, itemHref, type Item, type ItemQuery, type Library } from '@/lib/api';
import { resolveRows, rowHref, rowQuery, usePrefs, type ResolvedRow } from '@/lib/home';
import { runtime } from '@/lib/format';
import { Hero, MetaLine } from '@/components/Hero';
import { MediaRow } from '@/components/MediaRow';
import { EditHomeDialog } from '@/components/EditHome';
import { EmptyState, Page } from '@/components/Page';
import { useLibraries } from '@/components/Sidebar';
import { usePlayer } from '@/components/Player';
import { Button } from '@/components/ui/button';

function useRecent(lib: Library) {
  return useQuery({
    queryKey: ['items', { library_id: lib.id, sort: 'added', limit: 24 }],
    queryFn: () => api.items({ library_id: lib.id, kind: lib.kind === 'shows' ? 'series' : 'movie', sort: 'added', limit: 24 }),
  });
}

function LibraryRow({ lib }: { lib: Library }) {
  const { data } = useRecent(lib);
  return <MediaRow title={`Recently Added in ${lib.name}`} items={data?.items ?? []} href={`/library/${lib.id}`} />;
}

function QueryRow({ title, query, href }: { title: string; query: ItemQuery; href?: string }) {
  const { data } = useQuery({ queryKey: ['items', query], queryFn: () => api.items(query) });
  return <MediaRow title={title} items={data?.items ?? []} href={href} />;
}

function HomeRowView({ row }: { row: ResolvedRow }) {
  const continueWatching = useQuery({ queryKey: ['continue'], queryFn: api.continueWatching, enabled: row.type === 'continue' });
  const nextUp = useQuery({ queryKey: ['next-up'], queryFn: api.nextUp, enabled: row.type === 'next-up' });
  switch (row.type) {
    case 'continue':
      return <MediaRow title={row.title} items={continueWatching.data ?? []} landscape />;
    case 'next-up':
      return <MediaRow title={row.title} items={nextUp.data ?? []} landscape />;
    case 'recent':
      return <LibraryRow lib={row.library} />;
    case 'seasonal':
      return row.season ? <QueryRow title={row.season.title} query={{ ...row.season.query, limit: 24 }} /> : null;
    case 'custom': {
      const f = row.filter;
      // The library page can show the same list unless the row uses filters only rows have.
      const linkable = !f.added_within && !f.min_runtime && !f.max_runtime && !f.min_rating;
      return <QueryRow title={row.title} query={rowQuery(f, row.library)} href={linkable ? rowHref(f) : undefined} />;
    }
  }
}

export function Home() {
  const { data: libraries, isLoading } = useLibraries();
  const prefs = usePrefs();
  const [editing, setEditing] = useState(false);
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
    <Page className="pb-16">
      {featuredQuery.data ? <Featured item={featuredQuery.data} /> : <div className="h-10" />}
      <div className="relative z-10 -mt-6 space-y-10">
        {rows
          .filter((r) => !r.hidden)
          .map((r) => (
            <HomeRowView key={r.id} row={r} />
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
        <h1 className="text-4xl font-bold leading-[1.05] md:text-6xl">{item.title}</h1>
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
