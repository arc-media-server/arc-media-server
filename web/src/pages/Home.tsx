import { useMemo } from 'react';
import { Link } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { FolderPlus, Info, Play } from 'lucide-react';
import { api, backdropUrl, itemHref, type Item, type Library } from '@/lib/api';
import { runtime } from '@/lib/format';
import { Hero, MetaLine } from '@/components/Hero';
import { MediaRow } from '@/components/MediaRow';
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

export function Home() {
  const { data: libraries, isLoading } = useLibraries();
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
          Add a movie or TV folder to the <code className="rounded bg-surface-2 px-1">libraries</code> section of{' '}
          <code className="rounded bg-surface-2 px-1">ams.yaml</code> and restart the server to get started.
        </EmptyState>
      </Page>
    );
  }

  return (
    <Page className="pb-16">
      {featuredQuery.data ? <Featured item={featuredQuery.data} /> : <div className="h-10" />}
      <div className="relative z-10 -mt-6 space-y-10">
        {libraries.map((lib) => (
          <LibraryRow key={lib.id} lib={lib} />
        ))}
      </div>
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
            <Button variant="primary" size="lg" onClick={() => play({ title: item.title, src: item.stream_url! })}>
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
