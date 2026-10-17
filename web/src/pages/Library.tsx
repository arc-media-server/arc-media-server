import { forwardRef, useEffect, useRef, type ComponentPropsWithoutRef, type ReactNode } from 'react';
import { useParams, useSearchParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowUpDown, ChevronDown, FolderSearch, Loader2, RefreshCw, X } from 'lucide-react';
import { api, type ItemQuery, type ItemSort, type Library, type Resolution } from '@/lib/api';
import { cn } from '@/lib/utils';
import { plural } from '@/lib/format';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';
import { useLibraries } from '@/components/Sidebar';
import { useIsAdmin } from '@/lib/auth';
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
  const sort = (params.get('sort') as ItemSort) in sorts ? (params.get('sort') as ItemSort) : 'title';
  const filters = {
    genre: params.get('genre') ?? undefined,
    decade: Number(params.get('decade')) || undefined,
    resolution: (params.get('resolution') as Resolution) || undefined,
    unwatched: params.get('unwatched') === '1' || undefined,
  };
  const filtered = Object.values(filters).some(Boolean);
  const { data: libraries } = useLibraries();
  const lib = libraries?.find((l) => l.id === Number(id));
  const admin = useIsAdmin();

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
        subtitle={items.data && plural(items.data.total, lib?.kind === 'shows' ? 'show' : 'movie')}
        actions={
          lib && (
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
              {admin && <ScanButton lib={lib} />}
            </>
          )
        }
      />
      {lib && facets.data && (
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
      <div className="px-6 md:px-10">
        {items.error ? (
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
          <PosterGrid items={items.data.items} />
        )}
      </div>
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
