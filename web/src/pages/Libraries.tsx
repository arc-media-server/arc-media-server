import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { FolderPlus, Loader2, Pencil, Plus, RefreshCw } from 'lucide-react';
import { kindIcon, kindUnit } from '@/lib/kinds';
import { api, type Library } from '@/lib/api';
import * as fmt from '@/lib/format';
import { LibraryWorkControls } from '@/components/LibraryWork';
import { LibraryEditorDialog } from '@/components/LibraryEditor';
import { EmptyState, ErrorNotice, ErrorState, Page, PageHeader } from '@/components/Page';
import { useLibraries } from '@/components/Sidebar';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

export function LibrariesPage() {
  const qc = useQueryClient();
  const query = useLibraries();
  const { data: libraries } = query;
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = useState<Library | 'new' | null>(null);
  const [removing, setRemoving] = useState<Library | null>(null);
  const [scanErrors, setScanErrors] = useState<Record<number, string>>({});
  const [requested, setRequested] = useState<number[]>([]);
  const [scanSummary, setScanSummary] = useState('');
  const scan = useMutation({
    mutationFn: async ({ targets }: { targets: Library[]; all: boolean }) => Promise.allSettled(targets.map(lib => api.scanLibrary(lib.id))),
    onMutate: ({ targets }) => {
      setScanSummary('');
      setScanErrors(previous => Object.fromEntries(Object.entries(previous).filter(([id]) => !targets.some(lib => lib.id === Number(id)))));
      setRequested(previous => previous.filter(id => !targets.some(lib => lib.id === id)));
    },
    onSuccess: (results, { targets, all }) => {
      const accepted: number[] = [];
      const errors: Record<number, string> = {};
      results.forEach((result, i) => {
        if (result.status === 'fulfilled') accepted.push(targets[i].id);
        else errors[targets[i].id] = result.reason instanceof Error ? result.reason.message : 'The scan could not be requested. Try again.';
      });
      setRequested(previous => [...previous, ...accepted]);
      setScanErrors(previous => ({ ...previous, ...errors }));
      if (all) setScanSummary(`Scan requested for ${fmt.plural(accepted.length, 'library', 'libraries')}.${Object.keys(errors).length ? ` ${fmt.plural(Object.keys(errors).length, 'library', 'libraries')} could not start. Retry below.` : ''}`);
      // Show the server's returned status immediately, then poll as usual while it runs.
      qc.setQueryData<Library[]>(['libraries'], previous => previous?.map(lib => {
        const index = targets.findIndex(target => target.id === lib.id);
        const result = results[index];
        return result?.status === 'fulfilled' ? { ...lib, scan: result.value } : lib;
      }));
      void qc.invalidateQueries({ queryKey: ['activity'] });
      return qc.invalidateQueries({ queryKey: ['libraries'] });
    },
  });
  const requestScans = (targets: Library[], all = false) => {
    const idle = targets.filter(lib => !lib.scan.running);
    if (!scan.isPending && idle.length) scan.mutate({ targets: idle, all });
  };
  const allScanning = !!libraries?.length && libraries.every(lib => lib.scan.running);

  // /libraries?add=1 opens the add dialog straight away (from the welcome screen).
  useEffect(() => {
    if (params.get('add')) {
      setEditing('new');
      setParams({}, { replace: true });
    }
  }, [params, setParams]);

  const remove = useMutation({
    mutationFn: (lib: Library) => api.deleteLibrary(lib.id),
    onSuccess: () => {
      setRemoving(null);
      return qc.invalidateQueries();
    },
  });

  if (query.error && !libraries) return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
  return (
    <Page className="pb-16">
      <PageHeader
        title="Libraries"
        subtitle="The folders Arc looks in. New files show up on their own; Arc never changes them."
        actions={
          <>
          {!!libraries?.length && <Button variant="secondary" size="sm" disabled={scan.isPending || allScanning} onClick={() => requestScans(libraries, true)}>
            {scan.isPending ? <Loader2 className="animate-spin" /> : <RefreshCw />} Scan all libraries
          </Button>}
          <Button variant="secondary" size="sm" onClick={() => setEditing('new')}>
            <Plus /> Add Library
          </Button>
          </>
        }
      />
      <div className="space-y-4 px-6 md:px-10">
        {!!libraries?.length && <p className="text-sm text-muted">Scans check for new or changed files. Follow progress in Activity. Libraries already scanning continue their current scan.</p>}
        {scanSummary && <p role="status" className="text-sm text-muted">{scanSummary}</p>}
        {allScanning && <p role="status" className="text-sm text-accent">All libraries are already scanning.</p>}
        {libraries?.length === 0 && (
          <EmptyState icon={<FolderPlus />} title="No libraries yet">
            Add a folder of movies or TV shows to get started.
          </EmptyState>
        )}
        {libraries && libraries.length > 0 && (
          <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
            {libraries.map((lib) => (
              <LibraryRow key={lib.id} lib={lib} onEdit={() => setEditing(lib)} onRemove={() => setRemoving(lib)}
                onScan={() => requestScans([lib])} scanDisabled={scan.isPending} scanPending={scan.isPending && !!scan.variables?.targets.some(target => target.id === lib.id)}
                scanError={scanErrors[lib.id]} scanRequested={requested.includes(lib.id)} />
            ))}
          </div>
        )}
      </div>

      <LibraryEditorDialog
        library={editing && editing !== 'new' ? editing : undefined}
        open={!!editing}
        onOpenChange={(o) => !o && setEditing(null)}
      />
      <ConfirmDialog
        open={!!removing}
        onOpenChange={(o) => {
          if (!o) {
            setRemoving(null);
            remove.reset();
          }
        }}
        title={`Remove ${removing?.name ?? ''}?`}
        confirmLabel="Remove"
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => removing && remove.mutate(removing)}
      >
        <p>
          Arc forgets everything about this library: its {removing?.kind === 'shows' ? 'shows, episode orders' : kindUnit(removing?.kind)[1]} and anything
          you've edited. Your files aren't touched, and you can add the folders again at any time.
        </p>
        <p>A backup is made first, so you can undo this from Backups.</p>
      </ConfirmDialog>
    </Page>
  );
}

function LibraryRow({ lib, onEdit, onRemove, onScan, scanDisabled, scanPending, scanError, scanRequested }: {
  lib: Library; onEdit: () => void; onRemove: () => void; onScan: () => void;
  scanDisabled: boolean; scanPending: boolean; scanError?: string; scanRequested: boolean;
}) {
  const Icon = kindIcon(lib.kind);
  const what = kindUnit(lib.kind);
  return (
    <section aria-label={lib.name} className="grid grid-cols-[40px_minmax(0,1fr)] items-start gap-x-4 gap-y-3 px-4 py-3.5 sm:flex sm:items-center">
      <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-surface-2 text-muted">
        <Icon className="size-5" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold">
          {lib.name} <span className="font-normal text-muted">· {fmt.plural(lib.count, what[0], what[1])}</span>
        </p>
        {lib.paths.map((p) => (
          <p key={p} className="truncate font-mono text-[11px] text-subtle" title={p}>
            {p}
          </p>
        ))}
        {lib.scan.running && (
          <p role="status" className="mt-0.5 flex items-center gap-1.5 text-xs text-accent">
            <Loader2 className="size-3 animate-spin" /> {fmt.scanProgress(lib.scan)}
          </p>
        )}
        {!lib.scan.running && scanPending && <p role="status" className="mt-1 flex items-center gap-1.5 text-xs text-muted"><Loader2 className="size-3 animate-spin" /> Requesting a scan…</p>}
        {!lib.scan.running && lib.scan.stopped && <p role="status" className="mt-1 text-xs text-muted">Stopped. Completed changes were kept.</p>}
        {!lib.scan.running && !lib.scan.stopped && scanRequested && <p role="status" className="mt-1 text-xs text-muted">Scan requested. See Activity for results.</p>}
        {!lib.scan.running && lib.scan.error && <p role="alert" className="mt-1 text-xs text-warning">{lib.scan.error}</p>}
        {scanError && <div className="mt-2"><ErrorNotice error={scanError} onRetry={scanDisabled || lib.scan.running ? undefined : onScan} /></div>}
      </div>
      <div className="col-start-2 flex flex-wrap items-center gap-2 sm:shrink-0">
        <LibraryWorkControls library={lib} onScan={onScan} scanDisabled={scanDisabled} onRemove={onRemove}>
          <Button variant="secondary" size="sm" onClick={onEdit}>
            <Pencil /> Edit
          </Button>
        </LibraryWorkControls>
      </div>
    </section>
  );
}
