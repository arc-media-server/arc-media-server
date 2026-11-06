import { useTranslation } from 'react-i18next';
import { useState, type ReactNode } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Gauge, Layers, ListMusic, ListStart, ListVideo, Loader2, MoreHorizontal, Pencil, ScanSearch, SearchCheck, SearchX, Split, RefreshCw, Users } from 'lucide-react';
import { useWatchTogether } from '@/components/WatchTogether';
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

type Dialog = 'edit' | 'match' | 'unmatch' | 'collection' | 'split' | 'optimize' | null;

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
      <EditDetailsDialog item={item} open={open === 'edit'} onOpenChange={(o) => setOpen(o ? 'edit' : null)} />
      <FixMatchDialog item={item} open={open === 'match'} onOpenChange={(o) => setOpen(o ? 'match' : null)} />
      <AddToCollectionDialog item={item} library={library} open={open === 'collection'} onOpenChange={(o) => setOpen(o ? 'collection' : null)} />
      <SplitDialog item={item} open={open === 'split'} onOpenChange={(o) => setOpen(o ? 'split' : null)} />
      <UnmatchDialog item={item} open={open === 'unmatch'} onOpenChange={(o) => setOpen(o ? 'unmatch' : null)} />
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
 * The "⋯" button on a movie, show or episode, as Plex's: Play next, Add to queue and Add to
 * Playlist for anyone who can play the title, any `extra` items (Media Info, an episode's
 * playback options and details), then the server owner's tools (a show's episode ordering is in
 * Edit, Metadata & ordering). `compact` is the small trigger an episode row uses.
 */
export function MoreMenu({ item, admin, playable, edition, onOpen, onRefresh, refreshing, onAnalyze, analyzing, extra, compact, label }: {
  item: Item; admin: boolean; playable: boolean; edition?: string; onOpen: (d: Exclude<Dialog, null>) => void;
  onRefresh?: () => void; refreshing?: boolean; onAnalyze?: () => void; analyzing?: boolean;
  extra?: ReactNode; compact?: boolean; label?: string;
}) {
  const { t } = useTranslation('titles');
  const queue = useQueueActions(item, edition);
  const [playlist, setPlaylist] = useState(false);
  if (!admin && !playable && !extra) return null;
  const name = label ?? t('titles:itemActions.more');
  return (
    <span className={compact ? 'inline-flex max-w-full flex-row-reverse flex-wrap items-center gap-2' : 'inline-flex max-w-full flex-wrap items-center gap-2'}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          {compact
            ? <Button variant="ghost" size="icon" className="size-8 shrink-0 [&_svg]:size-4" title={t('titles:itemActions.more')} aria-label={name} disabled={queue.busy}>
                {queue.busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
              </Button>
            : <Button variant="secondary" size="lg" className="w-12 px-0" title={name} aria-label={name} disabled={queue.busy}>
                {queue.busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
              </Button>}
        </DropdownMenuTrigger>
        <DropdownMenuContent align={compact ? 'end' : 'start'} className="min-w-52">
          <MoreMenuItems item={item} admin={admin} playable={playable} onOpen={onOpen} onRefresh={onRefresh} refreshing={refreshing}
            onAnalyze={onAnalyze} analyzing={analyzing} onQueue={(next) => void queue.add(next)} onPlaylist={() => setPlaylist(true)}
            extra={extra && <>{playable && <DropdownMenuSeparator />}{extra}</>} />
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
  const together = useWatchTogether();
  // Watch Together starts from a movie or an episode (a show: pick the episode first), as in Plex.
  const togetherOK = playable && together.available && (item.kind === 'movie' || item.kind === 'episode');
  return (
    <>
      {playable && (
        <>
          {togetherOK && (
            <DropdownMenuItem onSelect={() => together.inviteTo(item)}><Users className="size-4 text-muted" /> {t('titles:itemActions.watchTogether')}</DropdownMenuItem>
          )}
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
          {item.kind !== 'episode' && (
            <DropdownMenuItem onSelect={() => onOpen('collection')}>
              <Layers className="size-4 text-muted" /> {t('titles:itemActions.addToCollection')}
            </DropdownMenuItem>
          )}
          {!isLocalVideo(item) && item.kind !== 'episode' && (
            <DropdownMenuItem onSelect={() => onOpen('match')}>
              <SearchCheck className="size-4 text-muted" /> {t('titles:itemActions.fixMatch')}
            </DropdownMenuItem>
          )}
          {item.kind === 'series' && !isLocalVideo(item) && item.metadata_state !== 'unmatched' && (
            <DropdownMenuItem onSelect={() => onOpen('unmatch')}>
              <SearchX className="size-4 text-muted" /> {t('titles:itemActions.unmatch')}
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

/**
 * Unmatch (a show): back to what its file names say, with downloaded details and artwork, manual
 * edits and display orders reset. Files, watch history, playlists and manual collections stay.
 * Refreshing doesn't undo it; Fix Match identifies it again.
 */
function UnmatchDialog({ item, open, onOpenChange }: { item: Item; open: boolean; onOpenChange: (o: boolean) => void }) {
  const { t } = useTranslation('titles');
  const qc = useQueryClient();
  const unmatch = useMutation({
    mutationFn: () => api.unmatchItem(item.id),
    onSuccess: () => {
      for (const key of ['item', 'items', 'seasons', 'libraries', 'recent-shows']) qc.invalidateQueries({ queryKey: [key] });
      onOpenChange(false);
    },
  });
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('titles:itemActions.unmatchTitle', { title: item.title })}
      confirmLabel={t('titles:itemActions.unmatchConfirm')}
      destructive
      busy={unmatch.isPending}
      error={unmatch.error?.message}
      onConfirm={() => unmatch.mutate()}
    >
      <p>{t('titles:itemActions.unmatchResets')}</p>
      <p>{t('titles:itemActions.unmatchKeeps')}</p>
    </ConfirmDialog>
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
