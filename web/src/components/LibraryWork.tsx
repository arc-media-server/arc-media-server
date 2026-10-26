import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { MoreHorizontal, RefreshCw, ScanSearch, Trash2 } from 'lucide-react';
import { api, type Library, type ScanStatus } from '@/lib/api';
import { EmptyTrashDialog } from '@/components/EmptyTrash';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { ErrorNotice } from '@/components/Page';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

/** Explicit costly repairs stay separate from efficient discovery scans. */
export function LibraryWorkControls({ library }: { library: Library }) {
  const [operation, setOperation] = useState<'refresh' | 'analyze' | null>(null);
  const [trashOpen, setTrashOpen] = useState(false);
  const qc = useQueryClient();
  const updated = (scan: ScanStatus) => {
    qc.setQueryData<Library[]>(['libraries'], prior => prior?.map(lib => lib.id === library.id ? { ...lib, scan } : lib));
    void qc.invalidateQueries({ queryKey: ['libraries'] });
    void qc.invalidateQueries({ queryKey: ['activity'] });
  };
  const start = useMutation({ mutationFn: (kind: 'refresh' | 'analyze') => api.startLibraryWork(library.id, kind), onSuccess: scan => { updated(scan); setOperation(null); } });
  const stop = useMutation({ mutationFn: (job: number) => api.stopLibraryWork(library.id, job), onSuccess: updated });
  return <>
    {library.scan.running && !!library.scan.job_id && <Button variant="secondary" size="sm"
      aria-label={`Stop work in ${library.name}`} disabled={stop.isPending || library.scan.stopping}
      onClick={() => stop.mutate(library.scan.job_id!)}>{stop.isPending || library.scan.stopping ? 'Stopping…' : 'Stop'}</Button>}
    <DropdownMenu>
      <DropdownMenuTrigger asChild><Button variant="ghost" size="icon" aria-label={`Library actions for ${library.name}`}><MoreHorizontal /></Button></DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {library.kind !== 'videos' && <DropdownMenuItem disabled={library.scan.running || start.isPending} onSelect={() => { start.reset(); setOperation('refresh'); }}><RefreshCw /> Refresh all metadata…</DropdownMenuItem>}
        <DropdownMenuItem disabled={library.scan.running || start.isPending} onSelect={() => { start.reset(); setOperation('analyze'); }}><ScanSearch /> Reanalyze files…</DropdownMenuItem>
        <DropdownMenuItem disabled={library.scan.running || start.isPending} onSelect={() => setTrashOpen(true)}><Trash2 /> Empty trash…</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
    <EmptyTrashDialog libraryId={library.id} libraryName={library.name} open={trashOpen} onOpenChange={setTrashOpen} />
    <ConfirmDialog open={!!operation} onOpenChange={open => { if (!open && !start.isPending) { setOperation(null); start.reset(); } }}
      title={operation === 'refresh' ? 'Refresh all metadata?' : 'Reanalyze library files?'}
      confirmLabel={operation === 'refresh' ? 'Refresh all metadata' : 'Reanalyze files'} busy={start.isPending} error={start.error?.message}
      onConfirm={() => operation && start.mutate(operation)}>
      <p>{operation === 'refresh'
        ? `Fetch online details again for every known movie or series in ${library.name}, including recently refreshed titles. This can take time and make many provider requests. Locked edits and watch history are kept.`
        : `Read technical details again for all known present files in ${library.name}, including HDR and audio format checks that were previously unavailable. This can keep the server busy. Matches, watch history and manual markers are kept; new files are found with Scan.`}</p>
      <p>Follow progress or Stop in Activity. Completed changes are kept if stopped. Media files are never changed.</p>
    </ConfirmDialog>
    <ErrorNotice error={stop.error} onRetry={() => { if (stop.variables) stop.mutate(stop.variables); }} busy={stop.isPending} />
  </>;
}
