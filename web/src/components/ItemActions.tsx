import { useState } from 'react';
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
  const dialogs = item && (
    <>
      <EditDetailsDialog item={item} open={open === 'edit' || open === 'ordering'} initialTab={open === 'ordering' ? 'ordering' : undefined} onOpenChange={(o) => setOpen(o ? 'edit' : null)} />
      <FixMatchDialog item={item} open={open === 'match'} onOpenChange={(o) => setOpen(o ? 'match' : null)} />
      <AddToCollectionDialog item={item} library={library} open={open === 'collection'} onOpenChange={(o) => setOpen(o ? 'collection' : null)} />
      <SplitDialog item={item} open={open === 'split'} onOpenChange={(o) => setOpen(o ? 'split' : null)} />
      <OptimizeDialog items={[item]} open={open === 'optimize'} onOpenChange={(o) => setOpen(o ? 'optimize' : null)} />
    </>
  );
  return { open: setOpen, dialogs };
}

/**
 * Analyze from the "⋯" menu: rereads a title's files (every episode, for a show) in the
 * background, like Plex's Analyze. Progress shows in Activity; `notice` reports it or an error.
 */
export function useAnalyzeItem(item?: Item) {
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
        <p role="status" className="mt-3 text-sm text-muted">
          Analyzing {item?.kind === 'series' ? 'every episode' : 'its files'}. Progress shows in Activity.
        </p>
      )}
    </>
  );
  return { analyze: () => analyze.mutate(), analyzing: analyze.isPending, notice };
}

/**
 * The "⋯" button on a movie or show page, as Plex's: Play next, Add to queue and Add to Playlist
 * for anyone who can play the title, then the server owner's tools.
 */
export function MoreMenu({ item, admin, playable, edition, onOpen, onRefresh, refreshing, onAnalyze, analyzing }: {
  item: Item; admin: boolean; playable: boolean; edition?: string; onOpen: (d: Exclude<Dialog, null>) => void;
  onRefresh?: () => void; refreshing?: boolean; onAnalyze?: () => void; analyzing?: boolean;
}) {
  const queue = useQueueActions(item, edition);
  const [playlist, setPlaylist] = useState(false);
  if (!admin && !playable) return null;
  return (
    <span className="inline-flex max-w-full flex-wrap items-center gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="secondary" size="lg" className="w-12 px-0" title="More" aria-label="More" disabled={queue.busy}>
            {queue.busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {playable && (
            <>
              <DropdownMenuItem onSelect={() => void queue.add(true)}><ListStart className="size-4 text-muted" /> Play next</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => void queue.add(false)}><ListVideo className="size-4 text-muted" /> Add to queue</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setPlaylist(true)}><ListMusic className="size-4 text-muted" /> Add to Playlist…</DropdownMenuItem>
            </>
          )}
          {playable && admin && <DropdownMenuSeparator />}
          {admin && (
            <>
              {onRefresh && !isLocalVideo(item) && <DropdownMenuItem disabled={refreshing} onSelect={onRefresh}><RefreshCw className="size-4 text-muted" />{refreshing ? 'Refreshing metadata…' : 'Refresh metadata'}</DropdownMenuItem>}
              {onAnalyze && <DropdownMenuItem disabled={analyzing} onSelect={onAnalyze}><ScanSearch className="size-4 text-muted" />{analyzing ? 'Starting analysis…' : 'Analyze'}</DropdownMenuItem>}
              <DropdownMenuItem onSelect={() => onOpen('edit')}>
                <Pencil className="size-4 text-muted" /> Edit…
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => onOpen('collection')}>
                <Layers className="size-4 text-muted" /> Add to Collection…
              </DropdownMenuItem>
              {!isLocalVideo(item) && (
                <DropdownMenuItem onSelect={() => onOpen('match')}>
                  <SearchCheck className="size-4 text-muted" /> Fix Match…
                </DropdownMenuItem>
              )}
              {!isLocalVideo(item) && (
                <DropdownMenuItem onSelect={() => onOpen('optimize')}>
                  <Gauge className="size-4 text-muted" /> Optimize…
                </DropdownMenuItem>
              )}
              {splitVersions(item) > 1 && (
                <DropdownMenuItem onSelect={() => onOpen('split')}>
                  <Split className="size-4 text-muted" /> Split Apart…
                </DropdownMenuItem>
              )}
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <span role="status" className="text-xs text-muted empty:hidden">{queue.notice}</span>
      <ErrorNotice error={queue.error} />
      <AddToPlaylistDialog item={item} open={playlist} onOpenChange={setPlaylist} />
    </span>
  );
}

/** Split Apart: each version of a movie but the first becomes a title of its own. */
function SplitDialog({ item, open, onOpenChange }: { item: Item; open: boolean; onOpenChange: (o: boolean) => void }) {
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
      title="Split Apart"
      confirmLabel="Split Apart"
      busy={split.isPending}
      error={split.error?.message}
      onConfirm={() => split.mutate()}
    >
      {item.title} has {n} versions. Splitting gives the other {n - 1 === 1 ? 'one' : n - 1} a title of {n - 1 === 1 ? 'its' : 'their'} own, identified
      online from the file name. Use this when different films were grouped together by mistake. Watch history stays with {item.title}.
    </ConfirmDialog>
  );
}
