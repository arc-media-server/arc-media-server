import { useEffect, useRef } from 'react';
import { useParams, useSearchParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowUpDown, FolderSearch, Loader2, RefreshCw } from 'lucide-react';
import { api, type ItemQuery, type Library } from '@/lib/api';
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

const sorts = { title: 'Title', added: 'Recently Added', year: 'Release Year' } as const;
type Sort = keyof typeof sorts;

export function LibraryPage() {
  const { id } = useParams();
  const [params, setParams] = useSearchParams();
  const sort = (params.get('sort') as Sort) in sorts ? (params.get('sort') as Sort) : 'title';
  const { data: libraries } = useLibraries();
  const lib = libraries?.find((l) => l.id === Number(id));
  const admin = useIsAdmin();

  const query: ItemQuery = { library_id: Number(id), kind: lib?.kind === 'shows' ? 'series' : 'movie', sort, limit: 500 };
  const items = useQuery({ queryKey: ['items', query], queryFn: () => api.items(query), enabled: !!lib });

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
                  <DropdownMenuRadioGroup value={sort} onValueChange={(v) => setParams({ sort: v }, { replace: true })}>
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
      <div className="px-6 md:px-10">
        {items.error ? (
          <ErrorState error={items.error} />
        ) : !items.data ? (
          <PosterGridSkeleton />
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
