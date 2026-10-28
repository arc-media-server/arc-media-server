import { useTranslation } from 'react-i18next';
import { useState, type ReactNode } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { FolderSearch, MoreHorizontal, RefreshCw, ScanSearch, Trash2, X } from 'lucide-react';
import { api, type Library, type ScanStatus } from '@/lib/api';
import { EmptyTrashDialog } from '@/components/EmptyTrash';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { ErrorNotice } from '@/components/Page';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

/** Explicit costly repairs stay separate from efficient discovery scans. */
/** Stop while work runs, then the library's More menu: Scan, the costly repairs, Empty trash and
 * Remove (as Plex's library ⋯ menu). Edit sits beside it on the row. */
export function LibraryWorkControls({ library, onScan, scanDisabled, onRemove, children }: {
  library: Library; onScan: () => void; scanDisabled: boolean; onRemove: () => void; children?: ReactNode;
}) {
  const { t } = useTranslation('libraryManagement');
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
      aria-label={t('libraryManagement:libraryWork.stopLibrary', { name: library.name })} disabled={stop.isPending || library.scan.stopping}
      onClick={() => stop.mutate(library.scan.job_id!)}>{stop.isPending || library.scan.stopping ? t("libraryManagement:libraryWork.stopping") : t("libraryManagement:libraryWork.stop")}</Button>}
    {children}
    <DropdownMenu>
      <DropdownMenuTrigger asChild><Button variant="ghost" size="icon" aria-label={t('libraryManagement:libraryWork.actions', { name: library.name })}><MoreHorizontal /></Button></DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem disabled={scanDisabled || library.scan.running} onSelect={onScan}><FolderSearch /> {t("libraryManagement:libraryWork.scanLibraryFiles")}</DropdownMenuItem>
        {library.kind !== 'videos' && <DropdownMenuItem disabled={library.scan.running || start.isPending} onSelect={() => { start.reset(); setOperation('refresh'); }}><RefreshCw /> {t("libraryManagement:libraryWork.refreshAllMetadata")}</DropdownMenuItem>}
        <DropdownMenuItem disabled={library.scan.running || start.isPending} onSelect={() => { start.reset(); setOperation('analyze'); }}><ScanSearch /> {t("libraryManagement:libraryWork.reanalyzeFiles")}</DropdownMenuItem>
        <DropdownMenuItem disabled={library.scan.running || start.isPending} onSelect={() => setTrashOpen(true)}><Trash2 /> {t("libraryManagement:libraryWork.emptyTrash")}</DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onRemove}><X /> {t("libraryManagement:libraryWork.removeLibrary")}</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
    <EmptyTrashDialog libraryId={library.id} libraryName={library.name} open={trashOpen} onOpenChange={setTrashOpen} />
    <ConfirmDialog open={!!operation} onOpenChange={open => { if (!open && !start.isPending) { setOperation(null); start.reset(); } }}
      title={operation === 'refresh' ? t("libraryManagement:libraryWork.refreshAllMetadata2") : t("libraryManagement:libraryWork.reanalyzeLibraryFiles")}
      confirmLabel={operation === 'refresh' ? t("libraryManagement:libraryWork.refreshAllMetadata3") : t("libraryManagement:libraryWork.reanalyzeFiles2")} busy={start.isPending} error={start.error?.message}
      onConfirm={() => operation && start.mutate(operation)}>
      <p>{operation === 'refresh'
        ? t('libraryManagement:libraryWork.refreshDescription', { name: library.name })
        : t('libraryManagement:libraryWork.analyzeDescription', { name: library.name })}</p>
      <p>{t("libraryManagement:libraryWork.followProgressOrStop")}</p>
    </ConfirmDialog>
    <ErrorNotice error={stop.error} onRetry={() => { if (stop.variables) stop.mutate(stop.variables); }} busy={stop.isPending} />
  </>;
}
