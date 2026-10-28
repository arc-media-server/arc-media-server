import { useTranslation } from 'react-i18next';
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate, useNavigationType, useParams, useSearchParams } from 'react-router';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ListChecks, ChevronDown, MoreHorizontal, FolderSearch, Layers, Loader2, Plus, RefreshCw, Sparkles } from 'lucide-react';
import { LetterRail, letterOffsets } from '@/components/LetterRail';
import { api, type ItemQuery, type ItemSort, type Library, type Resolution, type Collection, type CollectionMode } from '@/lib/api';
import { ActiveFilters, FiltersButton, type FilterPatch } from '@/components/LibraryFilters';
import { cn } from '@/lib/utils';
import * as fmt from '@/lib/format';
import { kindUnit } from '@/lib/kinds';
import { sortDirections, sortLabels } from '@/lib/filter-labels';
import { countUnit } from '@/lib/browsing-labels';
import { EmptyState, ErrorState, ErrorNotice, Page, PageHeader } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';
import { VideoFolders } from '@/components/VideoFolders';
import { MediaRow } from '@/components/MediaRow';
import { SelectionBar } from '@/components/BulkEdit';
import { useLibraries } from '@/components/Sidebar';
import { useAuth, useIsAdmin } from '@/lib/auth';
import { CollectionDialog, CollectionGrid } from '@/components/Collections';
import { EditDetailsDialog } from '@/components/EditDetails';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

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
  const { t } = useTranslation('library');
  const sorts = sortLabels();
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
  // A letter from an older link (?letter=s) jumps there once the titles load.
  const letterParam = params.get('letter')?.toLowerCase() ?? '';
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

  const view = params.get('view') === 'collections' ? 'collections' : params.get('view') === 'recommended' ? 'recommended' : params.get('view') === 'folders' ? 'folders' : 'titles';
  const recommended = useQuery({ queryKey: ['recommended', Number(id)], queryFn: () => api.recommended(Number(id)), enabled: !!lib && view === 'recommended' });
  const collections = useQuery({ queryKey: ['collections', Number(id)], queryFn: () => api.collections(Number(id)), enabled: !!lib });
  const [newCollection, setNewCollection] = useState(false);
  const [editing, setEditing] = useState<string>();
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
  const query: ItemQuery = { library_id: Number(id), kind: lib?.kind === 'shows' ? 'series' : 'movie', sort, reverse, limit: 500, ...filters };
  const items = useInfiniteQuery({
    queryKey: ['items', 'browse', query],
    // The first page also says where each letter starts, for the A–Z rail.
    queryFn: ({ pageParam }) => api.items({ ...query, offset: pageParam, letters: pageParam === 0 && sort === 'title' ? 1 : undefined }),
    initialPageParam: 0,
    getNextPageParam: (last, _pages, offset) => {
      const next = (last.offset ?? offset) + last.items.length;
      return last.items.length && next < last.total ? next : undefined;
    },
    select: (data) => ({ total: data.pages[0].total, items: data.pages.flatMap((p) => p.items), letters: data.pages[0].letters }),
    enabled: !!lib,
  });
  useBrowsePosition(`${profileId}:${location.key}`, !!items.data);
  const offsets = useMemo(() => letterOffsets(items.data?.letters ?? [], reverse), [items.data?.letters, reverse]);
  const [jumped, setJumped] = useState<string>();
  const gridRef = useRef<HTMLDivElement>(null);
  // A jump's own scroll keeps its letter lit (the row it lands on can start with an earlier one).
  const jumpedAt = useRef(0);
  // The rail follows scrolling: the letter of the first title in view.
  useEffect(() => {
    const list = items.data?.items;
    if (!list || !offsets.size) return;
    const index = new Map(list.map((it, i) => [it.id, i]));
    const starts = [...offsets].sort((a, b) => a[1] - b[1]);
    let frame = 0;
    const update = () => {
      frame = 0;
      if (performance.now() - jumpedAt.current < 400) return;
      const box = gridRef.current?.getBoundingClientRect();
      if (!box) return;
      const card = document.elementFromPoint(box.left + 24, Math.max(box.top, 0) + 40)?.closest<HTMLElement>('[data-item]');
      const at = card ? index.get(card.dataset.item!) : undefined;
      if (at === undefined) return;
      let letter = starts[0][0];
      for (const [l, o] of starts) if (o <= at) letter = l;
      setJumped(letter);
    };
    const onScroll = () => { if (!frame) frame = requestAnimationFrame(update); };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => { window.removeEventListener('scroll', onScroll); cancelAnimationFrame(frame); };
  }, [items.data?.items, offsets]);
  const [jumpError, setJumpError] = useState<{ error: Error; retry: () => void }>();
  useEffect(() => { setJumped(undefined); setJumpError(undefined); }, [selectionScope]);
  // Jump to a letter in the whole list: load the pages up to its first title, then scroll there.
  const jumpTo = async (letter: string, offset: number) => {
    setJumped(letter);
    setJumpError(undefined);
    let loaded = items.data?.items ?? [];
    let more = items.hasNextPage;
    while (loaded.length <= offset && more) {
      const next = await items.fetchNextPage();
      if (next.error) {
        setJumpError({ error: next.error, retry: () => void jumpTo(letter, offset) });
        return;
      }
      loaded = next.data?.items ?? loaded;
      more = next.hasNextPage;
    }
    // Newly loaded titles appear a frame or two after their page arrives.
    let tries = 0;
    const scroll = () => {
      // A title shown inside a collection has no card of its own: use the next one that does.
      for (const it of loaded.slice(offset, offset + 50)) {
        const card = document.querySelector<HTMLElement>(`[data-item="${CSS.escape(it.id)}"]`);
        if (card) {
          jumpedAt.current = performance.now();
          setJumped(letter);
          card.scrollIntoView({ block: 'start' });
          return;
        }
      }
      if (++tries < 60) requestAnimationFrame(scroll);
    };
    requestAnimationFrame(scroll);
  };
  useEffect(() => {
    if (!letterParam || !items.data || sort !== 'title') return;
    const next = new URLSearchParams(params);
    next.delete('letter');
    setParams(next, { replace: true });
    const letter = /^[a-z]$/.test(letterParam) ? letterParam : '#';
    const offset = offsets.get(letter);
    if (offset !== undefined) void jumpTo(letter, offset);
    // Only when an older link brings a letter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [letterParam, !!items.data]);
  const facets = useQuery({ queryKey: ['facets', Number(id)], queryFn: () => api.facets(Number(id)), enabled: !!lib });

  // Change one URL parameter, keeping the rest.
  const setParam = (key: string, value?: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    if (profileId && (key === 'sort' || key === 'reverse')) {
      const saved = new URLSearchParams(); saved.set('sort', next.get('sort') || 'title'); if (next.has('reverse')) saved.set('reverse', '1');
      try { localStorage.setItem(`arc-browse:${profileId}:${id}`, saved.toString()); } catch { /* storage is optional */ }
    }
    setParams(next, { replace: true });
  };

  // Plex shows the rail whenever a library is sorted by title.
  const showRail = view === 'titles' && sort === 'title' && offsets.size > 0;

  if (libraryQuery.error && !libraries) return <ErrorState error={libraryQuery.error} onRetry={() => void libraryQuery.refetch()} />;
  if (libraries && !lib) return <ErrorState error={t('library:notFound')} />;

  const tabs = lib ? (
    [
      ['titles', lib.kind === 'shows' ? t('library:tabs.shows') : lib.kind === 'videos' ? t('library:tabs.videos') : t('library:tabs.movies')],
      ['recommended', t('library:tabs.recommended')],
      ['folders', t('library:tabs.folders')],
      ['collections', t('library:tabs.collections')],
    ] as const
  )
    .filter(([v]) => v !== 'collections' || admin || (collections.data?.length ?? 0) > 0)
    // Browsing by folder is for Other videos libraries (movies and shows are grouped by title).
    .filter(([v]) => v !== 'folders' || lib.kind === 'videos')
    // Home videos have nothing to recommend from.
    .filter(([v]) => v !== 'recommended' || lib.kind !== 'videos') : [];
  const changeFilters = (patch: FilterPatch) => {
    const next = new URLSearchParams(params);
    for (const [key, value] of Object.entries(patch)) {
      if (value) next.set(key, value);
      else next.delete(key);
    }
    setParams(next, { replace: true });
  };

  return (
    // While choosing, room at the end for the selection bar so it never covers Load more.
    <Page className={selecting ? 'pb-36' : 'pb-16'}>
      <PageHeader
        title={lib?.name ?? ''}
        subtitle={
          view === 'collections'
            ? collections.data && t('library:counts.collections', { count: collections.data.length })
            : view === 'folders'
              ? t('library:foldersDescription')
            : view === 'recommended'
              ? t('library:recommendedDescription')
              : items.data && countUnit(items.data.total, unit)
        }
        actions={
          tabs.length > 1 && (
            // The library's views, as one switch.
            <div className="flex rounded-full bg-surface p-1 ring-1 ring-border" role="tablist">
              {tabs.map(([v, label]) => (
                <button
                  key={v}
                  type="button"
                  role="tab"
                  aria-selected={view === v}
                  onClick={() => { const next = new URLSearchParams(params); next.delete('folder'); if (v === 'titles') next.delete('view'); else next.set('view', v); setParams(next, { replace: true }); }}
                  className={cn(
                    'rounded-full px-3.5 py-1.5 text-[13px] font-medium transition-colors',
                    view === v ? 'bg-foreground text-background' : 'text-muted hover:text-foreground',
                  )}
                >
                  {label}
                </button>
              ))}
            </div>
          )
        }
      />
      {editing && <EditTitle id={editing} onClose={() => setEditing(undefined)} />}
      {lib && <CollectionDialog library={lib} open={newCollection} onOpenChange={setNewCollection} onSaved={(c) => navigate(`/collection/${c.id}`)} />}
      {!!facets.error && <div className="px-6 md:px-10"><ErrorNotice error={facets.error} onRetry={() => void facets.refetch()} /></div>}
      {lib && view === 'titles' && (
        <div className="-mt-1 mb-6 flex flex-wrap items-center gap-2 px-6 md:px-10">
          <SortMenu sort={sort} reverse={reverse} onSort={(v) => setParam('sort', v === 'title' ? undefined : v)} onReverse={(v) => setParam('reverse', v ? '1' : undefined)} />
          <FiltersButton facets={facets.data} filters={filters} onChange={changeFilters} onClear={clearFilters} />
          <ActiveFilters facets={facets.data} filters={filters} onChange={changeFilters} />
          {admin && (
            <div className="ml-auto flex items-center gap-2">
              <LibraryMenu lib={lib} onSelect={selecting ? undefined : () => setSelecting(true)} />
            </div>
          )}
        </div>
      )}
      {lib && view === 'collections' && admin && (
        <div className="-mt-1 mb-6 flex px-6 md:px-10">
          <Button variant="secondary" size="sm" onClick={() => setNewCollection(true)}>
            <Plus /> {t('library:newCollection')}
          </Button>
        </div>
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
              <EmptyState icon={<Sparkles />} title={t('library:empty.recommendedTitle')}>
                {t('library:empty.recommendedDescription')}
              </EmptyState>
            </div>
          ) : (
            recommended.data.rows.map((row) => (
              <MediaRow key={row.title} title={row.title} items={row.items} href={row.genre ? `/library/${id}?genre=${encodeURIComponent(row.genre)}&unwatched=1` : undefined} />
            ))
          )}
        </div>
      )}
      <div className={cn('px-6 md:px-10', view === 'recommended' && 'hidden', showRail && 'flex items-start gap-2 pr-2 md:pr-4')}>
        <div ref={gridRef} className="min-w-0 flex-1">
        {jumpError && <ErrorNotice error={jumpError.error} onRetry={jumpError.retry} />}
        {view === 'folders' && lib ? (
          <VideoFolders library={lib} />
        ) : view === 'collections' ? (
          collections.error ? (
            <ErrorState error={collections.error} />
          ) : !collections.data ? (
            <PosterGridSkeleton />
          ) : collections.data.length === 0 ? (
            <EmptyState icon={<Layers />} title={t('library:empty.collectionsTitle')}>
              {lib?.kind === 'videos'
                ? t('library:empty.videoCollections')
                : lib?.kind === 'shows'
                ? t('library:empty.showCollections')
                : t('library:empty.movieCollections')}
            </EmptyState>
          ) : (
            <CollectionGrid collections={collections.data} unit={unit} />
          )
        ) : items.error && !items.data ? (
          <ErrorState error={items.error} />
        ) : !items.data ? (
          <PosterGridSkeleton />
        ) : items.data.items.length === 0 && filtered ? (
          <EmptyState icon={<FolderSearch />} title={t('library:empty.filteredTitle')}>
            <p>{t('library:empty.filteredDescription')}</p>
            <Button variant="secondary" className="mt-4" onClick={clearFilters}>{t('library:clearFilters')}</Button>
          </EmptyState>
        ) : items.data.items.length === 0 ? (
          <EmptyState icon={<FolderSearch />} title={t('library:empty.title')}>
            {lib?.scan.running ? t('library:empty.scanning') : t('library:empty.paths', { paths: lib?.paths.join(', ') })}
          </EmptyState>
        ) : (
          <PosterGrid
            items={items.data.items}
            collections={!selecting && !filtered && lib?.kind !== 'videos' ? gridCollections(collections.data ?? [], lib?.collection_grid ?? '') : undefined}
            collectionUnit={unit}
            landscape={lib?.kind === 'videos'}
            hover={admin ? {
              // As in Plex: a card's circle starts choosing titles with that one chosen.
              onSelect: (item, index) => { setSelecting(true); setSelected(new Set([item.id])); lastClicked.current = index; },
              onEdit: lib?.kind === 'videos' ? undefined : (item) => setEditing(item.id),
            } : {}}
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
                {items.isFetchingNextPage && <Loader2 className="animate-spin" />} {t('library:loadMore')}
              </Button>
            )}
          </div>
        )}
        </div>
        {showRail && (
          <LetterRail
            offsets={offsets}
            current={jumped}
            onJump={(letter, offset) => void jumpTo(letter, offset)}
            className="sticky top-3 h-[calc(100dvh-1.5rem)] max-h-[680px] shrink-0"
          />
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

/**
 * How the titles are sorted, and which way, as two small menus. The direction is named for the
 * sort (A–Z, Newest first…); shuffle has none.
 */
function SortMenu({ sort, reverse, onSort, onReverse }: { sort: ItemSort; reverse: boolean; onSort: (sort: ItemSort) => void; onReverse: (reverse: boolean) => void }) {
  const { t } = useTranslation('library');
  const sorts = sortLabels();
  const directions = sort === 'shuffle' ? undefined : sortDirections(sort);
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="secondary" size="sm" className="shrink-0" aria-label={t('library:sortLabel', { sort: sorts[sort] })}>
            {sorts[sort]} <ChevronDown className="opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-52">
          <DropdownMenuLabel>{t('library:sortBy')}</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={sort} onValueChange={(v) => onSort(v as ItemSort)}>
            {Object.entries(sorts).map(([k, label]) => (
              <DropdownMenuRadioItem key={k} value={k}>{label}</DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>
      {directions && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="secondary" size="sm" className="shrink-0" aria-label={t('library:orderLabel', { order: directions[reverse ? 1 : 0] })}>
              {directions[reverse ? 1 : 0]} <ChevronDown className="opacity-60" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-52">
            <DropdownMenuRadioGroup value={reverse ? 'reverse' : 'normal'} onValueChange={(v) => onReverse(v === 'reverse')}>
              <DropdownMenuRadioItem value="normal">{directions[0]}</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="reverse">{directions[1]}</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </>
  );
}

/** Edit from a card's pencil: the dialog opens once the title's full details have loaded. */
function EditTitle({ id, onClose }: { id: string; onClose: () => void }) {
  const item = useQuery({ queryKey: ['item', id], queryFn: () => api.item(id) });
  if (item.error) return <div className="fixed inset-x-4 bottom-4 z-50 mx-auto max-w-md"><ErrorNotice error={item.error} onRetry={() => void item.refetch()} /></div>;
  return item.data ? <EditDetailsDialog item={item.data} open onOpenChange={(o) => !o && onClose()} /> : null;
}

/**
 * The library's ⋯ menu (server owner): choosing titles to change together (cards also start it
 * on hover), and scanning by hand.
 */
function LibraryMenu({ lib, onSelect }: { lib: Library; onSelect?: () => void }) {
  const { t } = useTranslation('library');
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
    const timer = window.setInterval(() => qc.invalidateQueries({ queryKey: ['items'] }), 5000);
    return () => window.clearInterval(timer);
  }, [lib.scan.running, qc]);

  // Arc scans by itself (it watches the folders, and checks hourly), so scanning by hand sits in
  // the library's ⋯ menu as in Plex; a running scan still shows its progress here.
  return (
    <>
      {lib.scan.running && (
        <span role="status" className="flex max-w-[22rem] items-center gap-2 text-sm text-muted" title={fmt.scanProgress(lib.scan)}>
          <Loader2 className="size-4 shrink-0 animate-spin" /> <span className="truncate">{fmt.scanProgress(lib.scan)}</span>
        </span>
      )}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" aria-label={t('library:libraryActions', { name: lib.name })}><MoreHorizontal /></Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem disabled={!onSelect} onSelect={onSelect}>
            <ListChecks className="size-4 text-muted" /> {t('library:selectTitles')}
          </DropdownMenuItem>
          <DropdownMenuItem disabled={lib.scan.running || scan.isPending} onSelect={() => scan.mutate()}>
            <RefreshCw className="size-4 text-muted" /> {t('library:scanLibraryFiles')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ErrorNotice error={scan.error} onRetry={() => scan.mutate()} />
    </>
  );
}

/** The collections that show in the grid (Plex's collection mode), with whether they hide their titles. */
function gridCollections(collections: Collection[], libraryDefault: Library['collection_grid']) {
  return collections
    .map((c) => ({ collection: c, mode: (c.grid_mode || libraryDefault || 'items') as CollectionMode }))
    .filter((g) => g.mode !== 'items' && g.collection.item_ids?.length)
    .map((g) => ({ collection: g.collection, hideItems: g.mode === 'collection' }));
}
