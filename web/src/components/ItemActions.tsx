import { useTranslation } from 'react-i18next';
import { useState, type ReactNode } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Gauge, Layers, ListMusic, ListStart, ListVideo, Loader2, MoreHorizontal, Pencil, ScanSearch, SearchCheck, Split, RefreshCw } from 'lucide-react';
import { OptimizeDialog } from '@/components/Optimize';
import { api, type Item } from '@/lib/api';
import { isLocalVideo } from '@/lib/kinds';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { EditDetailsDialog } from '@/components/EditDetails';
import { FixMatchDialog } from '@/components/FixMatch';
import { AddToCollectionDialog } from '@/components/Collections';
import { useLibraries } from '@/components/Sidebar';
import { Button } from '@/components/ui/button';
import { ErrorNotice } from '@/components/Page';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { useQueueActions } from '@/components/QueueActions';
import { AddToPlaylistDialog } from '@/components/Playlists';

type Dialog = 'edit' | 'match' | 'collection' | 'split' | 'optimize' | 'ordering' | null;

/** Versions of a movie that could be split into titles of their own (parts of one version count once). */
export function splitVersions(item: Item) {
  return item.kind === 'movie' ? (item.files ?? []).filter((f) => f.part <= 1).length : 0;
}

/** The dialogs for correcting an item, and a function to open one. */
export function useItemDialogs(item: Item | undefined) {
  const [open, setOpen] = useState<Dialog>(null);
  const { data: libraries } = useLibraries();
  const library = libraries?.find((l) => l.id === item?.library_id);
  const dialogs = item && open !== null && (
    <>
      <EditDetailsDialog item={item} open={open === 'edit' || open === 'ordering'} initialTab={open === 'ordering' ? 'ordering' : undefined} onOpenChange={(o) => setOpen(o ? 'edit' : null)} />
      <FixMatchDialog item={item} open={open === 'match'} onOpenChange={(o) => setOpen(o ? 'match' : null)} />
      <AddToCollectionDialog item={item} library={library} open={open === 'collection'} onOpenChange={(o) => setOpen(o ? 'collection' : null)} />
      <SplitDialog item={item} open={open === 'split'} onOpenChange={(o) => setOpen(o ? 'split' : null)} />
      <OptimizeDialog items={[item]} open={open === 'optimize'} onOpenChange={(o) => setOpen(o ? 'optimize' : null)} />
    </>
  );
  return { open: setOpen, current: open, dialogs };
}

/**
 * Analyze from the "⋯" menu: rereads a title's files (every episode, for a show) in the
 * background, like Plex's Analyze. Progress shows in Activity; `notice` reports it or an error.
 */
export function useAnalyzeItem(item?: Item) {
  const { t } = useTranslation('titles');
  const qc = useQueryClient();
  const analyze = useMutation({
    mutationFn: () => api.analyzeItem(item!.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['activity'] });
      qc.invalidateQueries({ queryKey: ['libraries'] });
    },
  });
  const notice = (
    <>
      <ErrorNotice error={analyze.error} onRetry={() => analyze.mutate()} />
      {analyze.isSuccess && (
        <p role="status" className="mt-3 text-sm text-muted">          {item?.kind === 'series' ? t('titles:itemActions.analyzingEpisodes') : t('titles:itemActions.analyzingFiles')}</p>
      )}
    </>
  );
  const message = item?.kind === 'series' ? t('titles:itemActions.analyzingEpisodes') : t('titles:itemActions.analyzingFiles');
  return { analyze: () => analyze.mutate(), analyzing: analyze.isPending, notice, started: analyze.isSuccess, message, error: analyze.error };
}

/**
 * The "⋯" button on a movie or show page, as Plex's: Play next, Add to queue and Add to Playlist
 * for anyone who can play the title, then the server owner's tools.
 */
export function MoreMenu({ item, admin, playable, edition, onOpen, onRefresh, refreshing, onAnalyze, analyzing }: {
  item: Item; admin: boolean; playable: boolean; edition?: string; onOpen: (d: Exclude<Dialog, null>) => void;
  onRefresh?: () => void; refreshing?: boolean; onAnalyze?: () => void; analyzing?: boolean;
}) {
  const { t } = useTranslation('titles');
  const queue = useQueueActions(item, edition);
  const [playlist, setPlaylist] = useState(false);
  if (!admin && !playable) return null;
  return (
    <span className="inline-flex max-w-full flex-wrap items-center gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="secondary" size="lg" className="w-12 px-0" title={t('titles:itemActions.more')} aria-label={t('titles:itemActions.more')} disabled={queue.busy}>
            {queue.busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <MoreMenuItems item={item} admin={admin} playable={playable} onOpen={onOpen} onRefresh={onRefresh} refreshing={refreshing}
            onAnalyze={onAnalyze} analyzing={analyzing} onQueue={(next) => void queue.add(next)} onPlaylist={() => setPlaylist(true)} />
        </DropdownMenuContent>
      </DropdownMenu>
      <span role="status" className="text-xs text-muted empty:hidden">{queue.notice}</span>
      <ErrorNotice error={queue.error} />
      <AddToPlaylistDialog item={item} open={playlist} onOpenChange={setPlaylist} />
    </span>
  );
}

/**
 * The items of a title's "⋯" menu: queue and playlist actions for anyone who can play it, any
 * `extra` items, then the server owner's tools. Shared by the title page and poster cards.
 */
export function MoreMenuItems({ item, admin, playable, onOpen, onRefresh, refreshing, onAnalyze, analyzing, onQueue, onPlaylist, extra }: {
  item: Item; admin: boolean; playable: boolean; onOpen: (d: Exclude<Dialog, null>) => void;
  onRefresh?: () => void; refreshing?: boolean; onAnalyze?: () => void; analyzing?: boolean;
  onQueue: (next: boolean) => void; onPlaylist: () => void; extra?: ReactNode;
}) {
  const { t } = useTranslation('titles');
  return (
    <>
      {playable && (
        <>
          <DropdownMenuItem onSelect={() => onQueue(true)}><ListStart className="size-4 text-muted" /> {t('titles:queueActions.playNext')}</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => onQueue(false)}><ListVideo className="size-4 text-muted" /> {t('titles:queueActions.addToQueue')}</DropdownMenuItem>
          <DropdownMenuItem onSelect={onPlaylist}><ListMusic className="size-4 text-muted" /> {t('titles:itemActions.addToPlaylist')}</DropdownMenuItem>
        </>
      )}
      {extra}
      {(playable || extra) && admin && <DropdownMenuSeparator />}
      {admin && (
        <>
          {onRefresh && !isLocalVideo(item) && <DropdownMenuItem disabled={refreshing} onSelect={onRefresh}><RefreshCw className="size-4 text-muted" />{refreshing ? t('titles:itemActions.refreshingMetadata') : t('titles:itemActions.refreshMetadata')}</DropdownMenuItem>}
          {onAnalyze && <DropdownMenuItem disabled={analyzing} onSelect={onAnalyze}><ScanSearch className="size-4 text-muted" />{analyzing ? t('titles:itemActions.startingAnalysis') : t('titles:itemActions.analyze')}</DropdownMenuItem>}
          <DropdownMenuItem onSelect={() => onOpen('edit')}>
            <Pencil className="size-4 text-muted" /> {t('titles:itemActions.edit')}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => onOpen('collection')}>
            <Layers className="size-4 text-muted" /> {t('titles:itemActions.addToCollection')}
          </DropdownMenuItem>
          {!isLocalVideo(item) && (
            <DropdownMenuItem onSelect={() => onOpen('match')}>
              <SearchCheck className="size-4 text-muted" /> {t('titles:itemActions.fixMatch')}
            </DropdownMenuItem>
          )}
          {!isLocalVideo(item) && (
            <DropdownMenuItem onSelect={() => onOpen('optimize')}>
              <Gauge className="size-4 text-muted" /> {t('titles:itemActions.optimize')}
            </DropdownMenuItem>
          )}
          {splitVersions(item) > 1 && (
            <DropdownMenuItem onSelect={() => onOpen('split')}>
              <Split className="size-4 text-muted" /> {t('titles:itemActions.splitApart')}
            </DropdownMenuItem>
          )}
        </>
      )}
    </>
  );
}

/** Split Apart: each version of a movie but the first becomes a title of its own. */
function SplitDialog({ item, open, onOpenChange }: { item: Item; open: boolean; onOpenChange: (o: boolean) => void }) {
  const { t } = useTranslation('titles');
  const qc = useQueryClient();
  const split = useMutation({
    mutationFn: () => api.splitItem(item.id),
    onSuccess: () => {
      for (const key of ['item', 'items', 'libraries', 'activity']) qc.invalidateQueries({ queryKey: [key] });
      onOpenChange(false);
    },
  });
  const n = splitVersions(item);
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("titles:itemActions.splitTitle")}
      confirmLabel={t("titles:itemActions.splitTitle")}
      busy={split.isPending}
      error={split.error?.message}
      onConfirm={() => split.mutate()}
    >
      {t('titles:itemActions.splitExplanation', { count: n - 1, versions: n, title: item.title })}
    </ConfirmDialog>
  );
}
