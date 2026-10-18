import { forwardRef, useEffect, useRef, useState, type ComponentPropsWithoutRef, type ReactNode } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ListChecks, ArrowUpDown, ChevronDown, FolderSearch, Layers, Loader2, Plus, RefreshCw, X, Sparkles } from 'lucide-react';
import { api, type ItemQuery, type ItemSort, type Library, type Resolution } from '@/lib/api';
import { cn } from '@/lib/utils';
import { kindUnit } from '@/lib/kinds';
import { language, plural } from '@/lib/format';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';
import { MediaRow } from '@/components/MediaRow';
import { SelectionBar } from '@/components/BulkEdit';
import { useLibraries } from '@/components/Sidebar';
import { useIsAdmin } from '@/lib/auth';
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
  runtime: 'Length',
  shuffle: 'Shuffle',
};
export const resolutions: Record<Resolution, string> = { '4k': '4K', '1080': '1080p', '720': '720p', sd: 'SD' };

export function LibraryPage() {
  const { id } = useParams();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const sort = (params.get('sort') as ItemSort) in sorts ? (params.get('sort') as ItemSort) : 'title';
  const filters = {
    genre: params.get('genre') ?? undefined,
    language: params.get('language') ?? undefined,
    label: params.get('label') ?? undefined,
    decade: Number(params.get('decade')) || undefined,
    resolution: (params.get('resolution') as Resolution) || undefined,
    unwatched: params.get('unwatched') === '1' || undefined,
  };
  const filtered = Object.values(filters).some(Boolean);
  const { data: libraries } = useLibraries();
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
  useEffect(() => {
    if (!selecting) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !document.querySelector('[role=dialog]')) stopSelecting();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [selecting]);
  const unit = kindUnit(lib?.kind)[0];
  const query: ItemQuery = { library_id: Number(id), kind: lib?.kind === 'shows' ? 'series' : 'movie', sort, limit: 500, ...filters };
  const items = useQuery({ queryKey: ['items', query], queryFn: () => api.items(query), enabled: !!lib });
  const facets = useQuery({ queryKey: ['facets', Number(id)], queryFn: () => api.facets(Number(id)), enabled: !!lib });

  // Change one URL parameter, keeping the rest.
  const setParam = (key: string, value?: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    setParams(next, { replace: true });
  };

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
              onClick={() => setParams(v === 'titles' ? {} : { view: v }, { replace: true })}
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
      {lib && view === 'titles' && facets.data && (
        <div className="scrollbar-none -mt-2 mb-6 flex gap-2 overflow-x-auto px-6 md:px-10">
          <FilterChip active={!!filters.unwatched} onClick={() => setParam('unwatched', filters.unwatched ? undefined : '1')}>
            Unwatched
          </FilterChip>
          <FilterMenu
            label="Genre"
            value={filters.genre}
            options={facets.data.genres.map((f) => [f.value, f.value, f.count])}
            onChange={(v) => setParam('genre', v)}
          />
          {facets.data.labels.length > 0 && (
            <FilterMenu
              label="Label"
              value={filters.label}
              options={facets.data.labels.map((f) => [f.value, f.value, f.count])}
              onChange={(v) => setParam('label', v)}
            />
          )}
          {facets.data.languages.length > 1 && (
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
          {facets.data.resolutions.length > 1 && (
            <FilterMenu
              label="Quality"
              value={filters.resolution}
              options={facets.data.resolutions.map((f) => [f.value, resolutions[f.value as Resolution] ?? f.value, f.count])}
              onChange={(v) => setParam('resolution', v)}
            />
          )}
          {filtered && (
            <Button variant="ghost" size="sm" className="shrink-0" onClick={() => setParams(sort === 'title' ? {} : { sort }, { replace: true })}>
              <X /> Clear
            </Button>
          )}
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
        ) : items.error ? (
          <ErrorState error={items.error} />
        ) : !items.data ? (
          <PosterGridSkeleton />
        ) : items.data.items.length === 0 && filtered ? (
          <EmptyState icon={<FolderSearch />} title="Nothing matches">
            Try fewer filters.
          </EmptyState>
        ) : items.data.items.length === 0 ? (
          <EmptyState icon={<FolderSearch />} title="Nothing here yet">
            {lib?.scan.running ? 'Scanning your folders…' : `Arc didn't find anything in ${lib?.paths.join(', ')}.`}
          </EmptyState>
        ) : (
          <PosterGrid
            items={items.data.items}
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
  if (!options.length) return null;
  const chosen = options.find(([v]) => v === value);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <FilterChip active={!!chosen}>
          {chosen ? chosen[1] : label} <ChevronDown className="size-3.5 opacity-60" />
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

  if (lib.scan.running) {
    return (
      <Button variant="secondary" size="sm" disabled>
        <Loader2 className="animate-spin" /> Scanning · {lib.scan.files_seen} files
      </Button>
    );
  }
  return (
    <Button variant="secondary" size="sm" onClick={() => scan.mutate()} disabled={scan.isPending}>
      <RefreshCw /> Scan Library
    </Button>
  );
}
