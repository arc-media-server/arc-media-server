import { forwardRef, useEffect, useLayoutEffect, useRef, useState, type ComponentPropsWithoutRef, type ReactNode } from 'react';
import { useLocation, useNavigate, useNavigationType, useParams, useSearchParams } from 'react-router';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ListChecks, ArrowUpDown, ChevronDown, FolderSearch, Layers, Loader2, Plus, RefreshCw, X, Sparkles } from 'lucide-react';
import { api, type ItemQuery, type ItemSort, type Library, type Resolution, type Collection, type CollectionMode } from '@/lib/api';
import { AdvancedFilterFields } from '@/components/FilterFields';
import { cn } from '@/lib/utils';
import * as fmt from '@/lib/format';
import { kindUnit } from '@/lib/kinds';
import { language, plural } from '@/lib/format';
import { EmptyState, ErrorState, ErrorNotice, Page, PageHeader } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';
import { MediaRow } from '@/components/MediaRow';
import { SelectionBar } from '@/components/BulkEdit';
import { useLibraries } from '@/components/Sidebar';
import { useAuth, useIsAdmin } from '@/lib/auth';
import { CollectionDialog, CollectionGrid } from '@/components/Collections';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export const sorts: Record<ItemSort, string> = {
  title: 'Title',
  added: 'Recently Added',
  watched: 'Recently Watched',
  release: 'Release Date',
  year: 'Year',
  rating: 'Rating',
  imdb: 'IMDb Rating',
  critics: 'Rotten Tomatoes',
  runtime: 'Length',
  shuffle: 'Shuffle',
};
export const resolutions: Record<Resolution, string> = { '4k': '4K', '1080': '1080p', '720': '720p', sd: 'SD' };

// History-entry positions live only in this tab, scoped to the active profile. Keep
// them bounded; the query cache supplies the loaded pages when returning from a title.
const browsePositions = new Map<string, number>();
function useBrowsePosition(key: string, ready: boolean) {
  const navigation = useNavigationType();
  const restore = useRef<number | undefined>(undefined);
  useLayoutEffect(() => {
    restore.current = navigation === 'POP' ? browsePositions.get(key) : undefined;
    let lastY = window.scrollY;
    const remember = () => { lastY = window.scrollY; };
    window.addEventListener('scroll', remember, { passive: true });
    // Router links can navigate before the last scroll event is delivered. Capture
    // the departure position before their click handler replaces the page.
    window.addEventListener('click', remember, true);
    return () => {
      // Detach before the app's passive route-to-top effect can overwrite lastY.
      window.removeEventListener('scroll', remember);
      window.removeEventListener('click', remember, true);
      browsePositions.delete(key);
      browsePositions.set(key, lastY);
      if (browsePositions.size > 40) browsePositions.delete(browsePositions.keys().next().value!);
    };
  }, [key, navigation]);
  useEffect(() => {
    if (!ready || restore.current === undefined) return;
    const y = restore.current;
    // Run after the app's normal route-to-top effect and cached posters have mounted.
    const frame = requestAnimationFrame(() => { window.scrollTo(0, y); restore.current = undefined; });
    return () => cancelAnimationFrame(frame);
  }, [key, ready]);
}

export function LibraryPage() {
  const { id } = useParams();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const location = useLocation();
  const profileId = useAuth().data?.profile?.id;
  useEffect(() => {
    if (!profileId || params.toString()) return;
    try { const saved = localStorage.getItem(`arc-browse:${profileId}:${id}`); if (saved) setParams(new URLSearchParams(saved), { replace: true }); } catch { /* private browsing may disable storage */ }
    // Restore only when opening a library, not when clearing its filters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, profileId]);
  const sort = (params.get('sort') as ItemSort) in sorts ? (params.get('sort') as ItemSort) : 'title';
  const letterParam = params.get('letter')?.toLowerCase() ?? '';
  const letter = sort === 'title' && /^[a-z]$/.test(letterParam) ? letterParam : '';
  const filters = {
    genre: params.get('genre') ?? undefined,
    language: params.get('language') ?? undefined,
    label: params.get('label') ?? undefined,
    decade: Number(params.get('decade')) || undefined,
    resolution: (params.get('resolution') as Resolution) || undefined,
    unwatched: params.get('unwatched') === '1' || undefined,
    added_within: Number(params.get('added_within')) || undefined,
    min_runtime: Number(params.get('min_runtime')) || undefined,
    max_runtime: Number(params.get('max_runtime')) || undefined,
    min_rating: Number(params.get('min_rating')) || undefined,
  };
  const filtered = Object.values(filters).some(Boolean);
  const clearFilters = () => {
    const next = new URLSearchParams(params);
    for (const key of Object.keys(filters)) next.delete(key);
    setParams(next, { replace: true });
  };
  const reverse = params.get('reverse') === '1';
  const libraryQuery = useLibraries();
  const { data: libraries } = libraryQuery;
  const lib = libraries?.find((l) => l.id === Number(id));
  const admin = useIsAdmin();

  const view = params.get('view') === 'collections' ? 'collections' : params.get('view') === 'recommended' ? 'recommended' : 'titles';
  const recommended = useQuery({ queryKey: ['recommended', Number(id)], queryFn: () => api.recommended(Number(id)), enabled: !!lib && view === 'recommended' });
  const collections = useQuery({ queryKey: ['collections', Number(id)], queryFn: () => api.collections(Number(id)), enabled: !!lib });
  const [newCollection, setNewCollection] = useState(false);
  // Choosing titles to change together (admins).
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const lastClicked = useRef<number | null>(null);
  const stopSelecting = () => {
    setSelecting(false);
    setSelected(new Set());
    lastClicked.current = null;
  };
  const selectionScope = `${id}?${params.toString()}`;
  useEffect(() => {
    setSelecting(false);
    setSelected(new Set());
    lastClicked.current = null;
  }, [selectionScope]);
  useEffect(() => {
    if (!selecting) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !document.querySelector('[role=dialog]')) stopSelecting();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [selecting]);
  const unit = kindUnit(lib?.kind)[0];
  const query: ItemQuery = { library_id: Number(id), kind: lib?.kind === 'shows' ? 'series' : 'movie', sort, reverse, limit: 500, ...filters, ...(letter ? { jump: letter } : {}) };
  const items = useInfiniteQuery({
    queryKey: ['items', 'browse', query],
    queryFn: ({ pageParam }) => api.items({ ...query, jump: pageParam === 0 ? query.jump : undefined, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (last, _pages, offset) => {
      const next = (last.offset ?? offset) + last.items.length;
      return last.items.length && next < last.total ? next : undefined;
    },
    select: (data) => ({ total: data.pages[0].total, items: data.pages.flatMap((p) => p.items) }),
    enabled: !!lib,
  });
  useBrowsePosition(`${profileId}:${location.key}`, !!items.data);
  const facets = useQuery({ queryKey: ['facets', Number(id)], queryFn: () => api.facets(Number(id)), enabled: !!lib });

  // Change one URL parameter, keeping the rest.
  const setParam = (key: string, value?: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    if (key === 'sort' && value && value !== 'title') next.delete('letter');
    if (profileId && (key === 'sort' || key === 'reverse')) {
      const saved = new URLSearchParams(); saved.set('sort', next.get('sort') || 'title'); if (next.has('reverse')) saved.set('reverse', '1');
      try { localStorage.setItem(`arc-browse:${profileId}:${id}`, saved.toString()); } catch { /* storage is optional */ }
    }
    setParams(next, { replace: true });
  };

  if (libraryQuery.error && !libraries) return <ErrorState error={libraryQuery.error} onRetry={() => void libraryQuery.refetch()} />;
  if (libraries && !lib) return <ErrorState error="Library not found" />;

  return (
    <Page className="pb-16">
      <PageHeader
        title={lib?.name ?? ''}
        subtitle={
          view === 'collections'
            ? collections.data && plural(collections.data.length, 'collection')
            : view === 'recommended'
              ? 'Picked for you from what you watch'
              : items.data && plural(items.data.total, unit)
        }
        actions={
          lib &&
          (view === 'recommended' ? null : view === 'collections' ? (
            admin && (
              <Button variant="secondary" size="sm" onClick={() => setNewCollection(true)}>
                <Plus /> New Collection
              </Button>
            )
          ) : (
            <>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="sm">
                    <ArrowUpDown /> {sorts[sort]}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuRadioGroup value={sort} onValueChange={(v) => setParam('sort', v === 'title' ? undefined : v)}>
                    {Object.entries(sorts).map(([k, label]) => (
                      <DropdownMenuRadioItem key={k} value={k}>
                        {label}
                      </DropdownMenuRadioItem>
                    ))}
                  </DropdownMenuRadioGroup>
                </DropdownMenuContent>
              </DropdownMenu>
              {sort === 'title' && (
                <label className="flex items-center gap-2 text-sm text-muted">
                  <span>Jump to</span>
                  <select aria-label="Jump to letter" value={letter} onChange={(e) => { setParam('letter', e.target.value || undefined); window.scrollTo(0, 0); }} className="h-9 max-w-32 rounded-lg bg-surface px-2 text-sm text-foreground ring-1 ring-border focus-visible:outline-2 focus-visible:outline-accent">
                    <option value="">All titles</option>
                    {'abcdefghijklmnopqrstuvwxyz'.split('').map((l) => <option key={l} value={l}>{l.toUpperCase()}</option>)}
                  </select>
                </label>
              )}
              {sort !== 'shuffle' && <Button variant="ghost" size="sm" aria-label="Reverse sort direction" aria-pressed={reverse} onClick={() => setParam('reverse', reverse ? undefined : '1')}>↕ <span className="hidden sm:inline">Reverse</span></Button>}
              {admin && !selecting && (
                <Button variant="ghost" size="sm" onClick={() => setSelecting(true)}>
                  <ListChecks /> Select
                </Button>
              )}
              {admin && <ScanButton lib={lib} />}
            </>
          ))
        }
      />
      {lib && (
        <div className="-mt-3 mb-5 flex gap-1 px-6 md:px-10" role="tablist">
          {(
            [
              ['titles', lib.kind === 'shows' ? 'Shows' : lib.kind === 'videos' ? 'Videos' : 'Movies'],
              ['recommended', 'Recommended'],
              ['collections', 'Collections'],
            ] as const
          )
            .filter(([v]) => v !== 'collections' || admin || (collections.data?.length ?? 0) > 0)
            // Home videos have nothing to recommend from.
            .filter(([v]) => v !== 'recommended' || lib.kind !== 'videos')
            .map(([v, label]) => (
            <button
              key={v}
              type="button"
              role="tab"
              aria-selected={view === v}
              onClick={() => { const next = new URLSearchParams(params); if (v === 'titles') next.delete('view'); else next.set('view', v); setParams(next, { replace: true }); }}
              className={cn(
                'rounded-full px-3.5 py-1 text-[13px] font-medium transition-colors',
                view === v ? 'bg-foreground text-background' : 'text-muted hover:text-foreground',
              )}
            >
              {label}
            </button>
          ))}
        </div>
      )}
      {lib && <CollectionDialog library={lib} open={newCollection} onOpenChange={setNewCollection} onSaved={(c) => navigate(`/collection/${c.id}`)} />}
      {!!facets.error && <div className="px-6 md:px-10"><ErrorNotice error={facets.error} onRetry={() => void facets.refetch()} /></div>}
      {lib && view === 'titles' && facets.data && (
        <div className="-mt-2 mb-6 flex flex-wrap gap-2 px-6 md:px-10">
          <FilterChip active={!!filters.unwatched} onClick={() => setParam('unwatched', filters.unwatched ? undefined : '1')}>
            Unwatched
          </FilterChip>
          <FilterMenu
            label="Genre"
            value={filters.genre}
            options={facets.data.genres.map((f) => [f.value, f.value, f.count])}
            onChange={(v) => setParam('genre', v)}
          />
          {(facets.data.labels.length > 0 || !!filters.label) && (
            <FilterMenu
              label="Label"
              value={filters.label}
              options={facets.data.labels.map((f) => [f.value, f.value, f.count])}
              onChange={(v) => setParam('label', v)}
            />
          )}
          {(facets.data.languages.length > 1 || !!filters.language) && (
            <FilterMenu
              label="Language"
              value={filters.language}
              options={facets.data.languages.map((f) => [f.value, language(f.value), f.count])}
              onChange={(v) => setParam('language', v)}
            />
          )}
          <FilterMenu
            label="Decade"
            value={filters.decade ? String(filters.decade) : undefined}
            options={facets.data.decades.map((f) => [f.value, `${f.value}s`, f.count])}
            onChange={(v) => setParam('decade', v)}
          />
          {(facets.data.resolutions.length > 1 || !!filters.resolution) && (
            <FilterMenu
              label="Quality"
              value={filters.resolution}
              options={facets.data.resolutions.map((f) => [f.value, resolutions[f.value as Resolution] ?? f.value, f.count])}
              onChange={(v) => setParam('resolution', v)}
            />
          )}
          {filtered && (
            <Button variant="ghost" size="sm" className="shrink-0" onClick={clearFilters}>
              <X /> Clear filters
            </Button>
          )}
        </div>
      )}
      {lib && view === 'titles' && (
        <details className="mx-6 mb-5 rounded-xl bg-surface px-4 py-3 md:mx-10" open={filters.added_within || filters.min_runtime || filters.max_runtime || filters.min_rating ? true : undefined}>
          <summary className="cursor-pointer text-sm font-medium">More filters</summary>
          <div className="mt-3 grid gap-3 sm:grid-cols-3">
            <AdvancedFilterFields filter={{ library_id: lib.id, ...filters }} onChange={(f) => {
              const next = new URLSearchParams(params);
              for (const key of ['added_within', 'min_runtime', 'max_runtime', 'min_rating'] as const) {
                if (f[key]) next.set(key, String(f[key])); else next.delete(key);
              }
              setParams(next, { replace: true });
            }} />
          </div>
        </details>
      )}
      {view === 'recommended' && (
        <div className="space-y-10">
          {recommended.error ? (
            <div className="px-6 md:px-10">
              <ErrorState error={recommended.error} />
            </div>
          ) : !recommended.data ? (
            <div className="px-6 md:px-10">
              <PosterGridSkeleton count={6} />
            </div>
          ) : recommended.data.rows.length === 0 ? (
            <div className="px-6 md:px-10">
              <EmptyState icon={<Sparkles />} title="Nothing to recommend yet">
                As you watch, this fills with titles like the ones you've enjoyed, the best rated you haven't seen, and more of your favourite genres.
              </EmptyState>
            </div>
          ) : (
            recommended.data.rows.map((row) => (
              <MediaRow key={row.title} title={row.title} items={row.items} href={row.genre ? `/library/${id}?genre=${encodeURIComponent(row.genre)}&unwatched=1` : undefined} />
            ))
          )}
        </div>
      )}
      <div className={cn('px-6 md:px-10', view === 'recommended' && 'hidden')}>
        {view === 'titles' && letter && (
          <p role="status" className="mb-4 text-sm text-muted">Starting at {letter.toUpperCase()}{reverse ? ' · Z–A' : ' · A–Z'}. <button type="button" onClick={() => setParam('letter')} className="text-accent underline underline-offset-4">All titles</button></p>
        )}
        {view === 'collections' ? (
          collections.error ? (
            <ErrorState error={collections.error} />
          ) : !collections.data ? (
            <PosterGridSkeleton />
          ) : collections.data.length === 0 ? (
            <EmptyState icon={<Layers />} title="No collections yet">
              {lib?.kind === 'videos'
                ? 'Each folder of videos appears here as a collection. You can also make your own with New Collection.'
                : lib?.kind === 'shows'
                ? 'Make one with New Collection: pick shows yourself, or let a smart collection gather everything that matches a filter.'
                : 'Films TMDB groups together (like a trilogy) appear here by themselves. You can also make your own with New Collection.'}
            </EmptyState>
          ) : (
            <CollectionGrid collections={collections.data} unit={unit} />
          )
        ) : items.error && !items.data ? (
          <ErrorState error={items.error} />
        ) : !items.data ? (
          <PosterGridSkeleton />
        ) : items.data.items.length === 0 && letter && items.data.total > 0 ? (
          <EmptyState icon={<FolderSearch />} title="No titles in this direction">
            <p>Choose another letter or return to All titles.</p>
            <Button variant="secondary" className="mt-4" onClick={() => setParam('letter')}>All titles</Button>
          </EmptyState>
        ) : items.data.items.length === 0 && filtered ? (
          <EmptyState icon={<FolderSearch />} title="Nothing matches">
            <p>Try fewer filters.</p>
            <Button variant="secondary" className="mt-4" onClick={clearFilters}>Clear filters</Button>
          </EmptyState>
        ) : items.data.items.length === 0 ? (
          <EmptyState icon={<FolderSearch />} title="Nothing here yet">
            {lib?.scan.running ? 'Scanning your folders…' : `Arc didn't find anything in ${lib?.paths.join(', ')}.`}
          </EmptyState>
        ) : (
          <PosterGrid
            items={items.data.items}
            collections={!selecting && !filtered && !letter && lib?.kind !== 'videos' ? gridCollections(collections.data ?? [], lib?.collection_grid ?? '') : undefined}
            collectionUnit={unit}
            landscape={lib?.kind === 'videos'}
            select={
              selecting
                ? {
                    selected,
                    onToggle: (item, index, e) => {
                      const list = items.data!.items;
                      const last = lastClicked.current;
                      setSelected((prev) => {
                        const next = new Set(prev);
                        // Shift-click chooses everything between this and the last one clicked.
                        if (e.shiftKey && last !== null) {
                          const [from, to] = [Math.min(last, index), Math.max(last, index)];
                          for (const it of list.slice(from, to + 1)) next.add(it.id);
                        } else if (next.has(item.id)) next.delete(item.id);
                        else next.add(item.id);
                        return next;
                      });
                      lastClicked.current = index;
                    },
                  }
                : undefined
            }
          />
        )}
        {view === 'titles' && items.data && (
          <div className="mt-8 flex flex-col items-center gap-3">
            {items.error && <p role="alert" className="text-sm text-warning">{items.error.message}</p>}
            {items.hasNextPage && (
              <Button variant="secondary" disabled={items.isFetchingNextPage} onClick={() => void items.fetchNextPage()}>
                {items.isFetchingNextPage && <Loader2 className="animate-spin" />} Load more
              </Button>
            )}
          </div>
        )}
      </div>
      {selecting && lib && view === 'titles' && items.data && (
        <SelectionBar
          library={lib}
          items={items.data.items}
          selected={selected}
          onSelectAll={() => setSelected(new Set(items.data!.items.map((i) => i.id)))}
          onClear={() => setSelected(new Set())}
          onDone={stopSelecting}
        />
      )}
    </Page>
  );
}

// A forwardRef so it can open a dropdown menu.
const FilterChip = forwardRef<HTMLButtonElement, ComponentPropsWithoutRef<'button'> & { active: boolean; children: ReactNode }>(
  ({ active, className, children, ...props }, ref) => (
    <button
      ref={ref}
      type="button"
      aria-pressed={active}
      className={cn(
        'flex h-8 shrink-0 items-center gap-1.5 rounded-full px-3.5 text-[13px] font-medium ring-1 transition-colors',
        active ? 'bg-foreground text-background ring-foreground' : 'bg-surface-2/60 text-muted ring-border hover:text-foreground',
        className,
      )}
      {...props}
    >
      {children}
    </button>
  ),
);
FilterChip.displayName = 'FilterChip';

/** A chip that opens a list of values to filter by; shows the chosen one. */
function FilterMenu({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value?: string;
  options: [value: string, label: string, count: number][];
  onChange: (value?: string) => void;
}) {
  if (!options.length && !value) return null;
  const chosen = options.find(([v]) => v === value);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <FilterChip active={!!value} className="max-w-full" aria-label={value ? `${label}: ${chosen?.[1] ?? value}` : label}>
          <span className="truncate">{chosen?.[1] ?? value ?? label}</span> <ChevronDown className="size-3.5 opacity-60" />
        </FilterChip>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-60">
        <DropdownMenuRadioGroup value={value ?? ''} onValueChange={(v) => onChange(v || undefined)}>
          <DropdownMenuRadioItem value="">Any {label.toLowerCase()}</DropdownMenuRadioItem>
          {options.map(([v, l, n]) => (
            <DropdownMenuRadioItem key={v} value={v}>
              <span className="flex-1">{l}</span>
              <span className="text-xs text-subtle">{n}</span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function ScanButton({ lib }: { lib: Library }) {
  const qc = useQueryClient();
  const scan = useMutation({
    mutationFn: () => api.scanLibrary(lib.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['libraries'] }),
  });

  // Refresh what's shown when a scan finishes.
  const wasRunning = useRef(lib.scan.running);
  useEffect(() => {
    if (wasRunning.current && !lib.scan.running) {
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['health'] });
      qc.invalidateQueries({ queryKey: ['health-summary'] });
      qc.invalidateQueries({ queryKey: ['featured'] });
    }
    wasRunning.current = lib.scan.running;
  }, [lib.scan.running, qc]);
  // Titles appear as they're identified, without waiting for the whole scan.
  useEffect(() => {
    if (!lib.scan.running) return;
    const t = window.setInterval(() => qc.invalidateQueries({ queryKey: ['items'] }), 5000);
    return () => window.clearInterval(t);
  }, [lib.scan.running, qc]);

  if (lib.scan.running) {
    return (
      <Button variant="secondary" size="sm" disabled title={fmt.scanProgress(lib.scan)}>
        <Loader2 className="animate-spin" /> <span className="max-w-[22rem] truncate">{fmt.scanProgress(lib.scan)}</span>
      </Button>
    );
  }
  return (
    <Button variant="secondary" size="sm" onClick={() => scan.mutate()} disabled={scan.isPending}>
      <RefreshCw /> Scan Library
    </Button>
  );
}

/** The collections that show in the grid (Plex's collection mode), with whether they hide their titles. */
function gridCollections(collections: Collection[], libraryDefault: Library['collection_grid']) {
  return collections
    .map((c) => ({ collection: c, mode: (c.grid_mode || libraryDefault || 'items') as CollectionMode }))
    .filter((g) => g.mode !== 'items' && g.collection.item_ids?.length)
    .map((g) => ({ collection: g.collection, hideItems: g.mode === 'collection' }));
}
