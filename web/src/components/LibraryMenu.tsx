import { useTranslation } from 'react-i18next';
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { FolderSearch, ListChecks, Loader2, MoreHorizontal, Pencil, Users } from 'lucide-react';
import { api, type Library } from '@/lib/api';
import * as fmt from '@/lib/format';
import { ErrorNotice } from '@/components/Page';
import { LibraryEditorDialog } from '@/components/LibraryEditor';
import { LibraryWorkDialogs, LibraryWorkItems, useLibraryWork } from '@/components/LibraryWork';
import { ReorderLibrariesItem, quietMenuTrigger } from '@/components/LibraryReorder';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

/** A library's ⋯ actions; `compact` is the sidebar's quiet version, which can also start reordering. */
export function LibraryMenu({ lib, onSelect, selectable = false, compact = false, onReorder }: { lib: Library; onSelect?: () => void; selectable?: boolean; compact?: boolean; onReorder?: () => void }) {
  const { t } = useTranslation('library');
  const navigate = useNavigate();
  const qc = useQueryClient();
  const work = useLibraryWork(lib);
  const [editOpen, setEditOpen] = useState(false);
  const scan = useMutation({
    mutationFn: () => api.scanLibrary(lib.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['libraries'] }),
  });

  // Refresh what's shown when a scan finishes.
  const wasRunning = useRef(lib.scan.running);
  useEffect(() => {
    if (!compact && wasRunning.current && !lib.scan.running) {
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['health'] });
      qc.invalidateQueries({ queryKey: ['health-summary'] });
      qc.invalidateQueries({ queryKey: ['featured'] });
    }
    wasRunning.current = lib.scan.running;
  }, [compact, lib.scan.running, qc]);
  // Titles appear as they're identified, without waiting for the whole scan.
  useEffect(() => {
    if (compact || !lib.scan.running) return;
    const timer = window.setInterval(() => qc.invalidateQueries({ queryKey: ['items'] }), 5000);
    return () => window.clearInterval(timer);
  }, [compact, lib.scan.running, qc]);

  // Arc scans by itself (it watches the folders, and checks hourly), so scanning by hand sits in
  // the library's ⋯ menu as in Plex; a running scan still shows its progress here.
  return (
    <>
      {!compact && lib.scan.running && (
        <span role="status" className="flex max-w-[22rem] items-center gap-2 text-sm text-muted" title={fmt.scanProgress(lib.scan)}>
          <Loader2 className="size-4 shrink-0 animate-spin" /> <span className="truncate">{fmt.scanProgress(lib.scan)}</span>
        </span>
      )}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" data-library-menu={compact ? lib.id : undefined} className={compact ? quietMenuTrigger : undefined} aria-label={t('library:libraryActions', { name: lib.name })}><MoreHorizontal /></Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="[&_svg]:size-4 [&_svg]:shrink-0 [&_svg]:text-muted">
          {selectable && (
            <DropdownMenuItem disabled={!onSelect} onSelect={onSelect}>
              <ListChecks /> {t('library:selectTitles')}
            </DropdownMenuItem>
          )}
          <DropdownMenuItem disabled={lib.scan.running || scan.isPending} onSelect={() => scan.mutate()}>
            <FolderSearch /> {t('library:scanLibraryFiles')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => setEditOpen(true)}>
            <Pencil /> {t('libraryManagement:libraryWork.editLibrary')}
          </DropdownMenuItem>
          <LibraryWorkItems work={work} />
          <DropdownMenuItem onSelect={() => navigate('/profiles')}>
            <Users /> {t('libraryManagement:libraryWork.manageAccess')}
          </DropdownMenuItem>
          {onReorder && <>
            <DropdownMenuSeparator />
            <ReorderLibrariesItem onSelect={onReorder} />
          </>}
        </DropdownMenuContent>
      </DropdownMenu>
      <LibraryEditorDialog library={lib} open={editOpen} onOpenChange={setEditOpen} />
      <LibraryWorkDialogs work={work} />
      <ErrorNotice error={scan.error} onRetry={() => scan.mutate()} />
    </>
  );
}

/** The collections that show in the grid (Plex's collection mode), with whether they hide their titles. */