import { useTranslation } from 'react-i18next';
import { useEffect, useState, type MouseEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Bookmark, BookmarkCheck, Check, Circle, Loader2, MoreVertical, Pencil, Play } from 'lucide-react';
import { api, type Item } from '@/lib/api';
import { cn } from '@/lib/utils';
import { useIsAdmin } from '@/lib/auth';
import { MoreMenuItems, useAnalyzeItem, useItemDialogs } from '@/components/ItemActions';
import { AddToPlaylistDialog } from '@/components/Playlists';
import { useQueueActions } from '@/components/QueueActions';
import { useWatchStateChanged } from '@/components/WatchedButton';
import { nowPlayingFor, usePlayer } from '@/components/Player';
import { nextToPlay } from '@/lib/next-episode';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

/** What a card offers on hover: choosing it among others (server owner), and editing it. */
export interface CardHover {
  onSelect?: (e: MouseEvent) => void;
  onEdit?: () => void;
}

/** Shown while the card is hovered or has focus; each control answers the pointer quickly. */
const reveal =
  'cursor-pointer opacity-0 transition-[opacity,scale,background-color,color,box-shadow] duration-150 ease-out motion-reduce:transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-solid focus-visible:outline-white';

/** The small round buttons at the bottom (pencil, ⋯): a soft disc behind the icon on hover. */
const iconButton =
  'pointer-events-auto absolute bottom-1.5 flex size-8 items-center justify-center rounded-full text-white/90 drop-shadow-[0_1px_3px_rgb(0_0_0/0.6)] hover:bg-white/20 hover:text-white hover:drop-shadow-none hover:backdrop-blur-sm focus-visible:bg-white/20 focus-visible:text-white active:bg-white/30';

/**
 * The select circle, the same control on hover and while choosing titles: a crisp white ring on a
 * dark disc (so it reads on light and dark posters), the height of the unwatched count opposite.
 * Chosen, it fills with the accent and shows the tick.
 */
export function selectCircle(selected: boolean) {
  return cn(
    'flex size-6 items-center justify-center rounded-full shadow-[0_1px_4px_rgb(0_0_0/0.35)] ring-2 ring-inset',
    selected ? 'bg-accent-strong text-white ring-accent-strong' : 'bg-black/35 text-white/0 ring-white/95 backdrop-blur-sm',
  );
}

/** Hovering the circle (or tabbing to it): it fills with the accent, glows and previews the tick. */
const selectCircleHover =
  'hover:bg-accent-strong hover:text-white hover:ring-accent-strong hover:shadow-[0_0_0_3px_rgb(255_255_255/0.22),0_0_14px_2px_color-mix(in_srgb,var(--accent)_65%,transparent)] focus-visible:bg-accent-strong focus-visible:text-white focus-visible:ring-accent-strong active:scale-95 motion-reduce:active:scale-100';

/**
 * A soft shade over the artwork while a card's actions show, as Plex's, so the white controls read
 * on pale posters. It sits under the watched badge, inside the artwork.
 */
export function CardScrim() {
  return (
    <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_bottom,rgb(0_0_0/0.38),rgb(0_0_0/0.12)_32%,rgb(0_0_0/0.12)_62%,rgb(0_0_0/0.5))] opacity-0 transition-opacity duration-200 group-hover:opacity-100 group-focus-within:opacity-100 [@media(hover:none)]:hidden" />
  );
}

/**
 * A poster's hover actions, as Plex's: a small circle to start choosing titles, Play in the
 * middle, the pencil to edit and "⋯" for everything else (also on right-click). Touch screens,
 * which can't hover, keep the plain card.
 */
export function CardActions({ item, title, hover, landscape, menuOpen, onMenuOpen }: {
  item: Item;
  title: string;
  hover: CardHover;
  landscape?: boolean;
  menuOpen: boolean;
  onMenuOpen: (open: boolean) => void;
}) {
  const { t } = useTranslation('common');
  const play = usePlayer();
  const [starting, setStarting] = useState(false);
  const playable = item.kind === 'series' || !!item.stream_url;
  const start = async () => {
    if (item.kind !== 'series') return play(nowPlayingFor(item));
    setStarting(true);
    try {
      const ep = nextToPlay((await api.seasons(item.id)).seasons.flatMap((s) => s.episodes));
      if (ep) play(nowPlayingFor({ ...ep, series_title: item.title }));
    } finally {
      setStarting(false);
    }
  };
  return (
    <div
      className={cn(
        // Rides up with the artwork on hover, so the controls stay lined up with the badge.
        'pointer-events-none absolute inset-x-0 top-0 z-10 transition-transform duration-300 ease-smooth group-hover:-translate-y-1 motion-reduce:transition-none [@media(hover:none)]:hidden',
        landscape ? 'aspect-video' : 'aspect-[2/3]',
      )}
    >
      {hover.onSelect && (
        <button
          type="button"
          aria-label={t('common:card.select', { title })}
          onClick={hover.onSelect}
          className={cn(
            'pointer-events-auto absolute left-1.5 top-1.5',
            selectCircle(false),
            selectCircleHover,
            reveal,
          )}
        >
          <Check className="size-3.5" strokeWidth={3} />
        </button>
      )}
      {playable && (
        <button
          type="button"
          aria-label={t('common:card.play', { title })}
          onClick={() => void start()}
          disabled={starting}
          className={cn(
            'pointer-events-auto absolute left-1/2 top-1/2 flex size-12 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full bg-black/45 text-white shadow-[0_2px_12px_rgb(0_0_0/0.35)] ring-2 ring-inset ring-white/90 backdrop-blur-sm',
            'hover:scale-110 hover:bg-accent-strong hover:ring-accent-strong hover:shadow-[0_0_0_4px_rgb(255_255_255/0.2),0_4px_22px_4px_color-mix(in_srgb,var(--accent)_60%,transparent)] focus-visible:bg-accent-strong focus-visible:ring-accent-strong active:scale-100 motion-reduce:hover:scale-100',
            reveal,
          )}
        >
          {starting ? <Loader2 className="size-5 animate-spin" /> : <Play className="ml-0.5 size-5 fill-current" />}
        </button>
      )}
      {hover.onEdit && (
        <button
          type="button"
          aria-label={t('common:card.edit', { title })}
          onClick={hover.onEdit}
          className={cn(iconButton, 'left-1.5', reveal)}
        >
          <Pencil className="size-4" />
        </button>
      )}
      <CardMenu item={item} title={title} open={menuOpen} onOpenChange={onMenuOpen} />
    </div>
  );
}

/**
 * The card's "⋯": its menu (and the dialogs it opens) are only built once it's used, and kept
 * while an action from it finishes or a dialog is open. Results show as a short note here, so
 * they survive the menu closing.
 */
function CardMenu({ item, title, open, onOpenChange }: { item: Item; title: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation('common');
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState('');
  useEffect(() => {
    if (!note) return;
    const timer = window.setTimeout(() => setNote(''), 4000);
    return () => window.clearTimeout(timer);
  }, [note]);
  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange} modal={false}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t('common:card.more', { title })}
          className={cn(iconButton, 'right-1.5 data-[state=open]:bg-white/20 data-[state=open]:opacity-100', reveal)}
        >
          <MoreVertical className="size-4" />
        </button>
      </DropdownMenuTrigger>
      {(open || busy) && <CardMenuBody item={item} open={open} onBusy={setBusy} onNote={setNote} />}
      {note && (
        <p role="status" className="fixed inset-x-0 bottom-6 z-50 mx-auto w-fit max-w-[calc(100vw-32px)] rounded-full bg-foreground px-4 py-2 text-sm font-medium text-background shadow-2xl">
          {note}
        </p>
      )}
    </DropdownMenu>
  );
}

function CardMenuBody({ item, open, onBusy, onNote }: { item: Item; open: boolean; onBusy: (busy: boolean) => void; onNote: (note: string) => void }) {
  const { t } = useTranslation('titles');
  const admin = useIsAdmin();
  const queue = useQueueActions(item);
  const dialogs = useItemDialogs(item);
  const analysis = useAnalyzeItem(item);
  const changed = useWatchStateChanged();
  const qc = useQueryClient();
  const [playlist, setPlaylist] = useState(false);
  const fail = (e: unknown) => onNote(e instanceof Error ? e.message : String(e));
  const refresh = useMutation({
    mutationFn: async () => { if (item.kind === 'series') await api.refreshSeries(item.id); else await api.refreshItem(item.id); },
    onSuccess: () => { changed(); onNote(t('titles:cardMenu.refreshed')); },
    onError: fail,
  });
  const watched = item.kind === 'series' ? item.unwatched_count === 0 : !!item.user_data?.played;
  const played = useMutation({ mutationFn: () => api.setPlayed(item.id, !watched), onSuccess: changed, onError: fail });
  const [listed, setListed] = useState(!!item.user_data?.watchlisted);
  const watchlist = useMutation({
    mutationFn: (on: boolean) => api.setWatchlist(item.id, on),
    onMutate: (on) => setListed(on),
    onError: (e, on) => { setListed(!on); fail(e); },
    onSuccess: () => {
      for (const key of ['watchlist', 'item', 'items']) qc.invalidateQueries({ queryKey: [key] });
    },
  });
  // Stay built while a dialog is open or an action from the menu is still finishing.
  const showing = dialogs.current !== null || playlist || queue.busy || analysis.analyzing ||
    refresh.isPending || played.isPending || watchlist.isPending;
  useEffect(() => { onBusy(showing); }, [showing, onBusy]);
  useEffect(() => { if (queue.notice) onNote(queue.notice); }, [queue.notice, onNote]);
  useEffect(() => { if (queue.error) fail(queue.error); }, [queue.error]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (analysis.started) onNote(analysis.message); }, [analysis.started, analysis.message, onNote]);
  useEffect(() => { if (analysis.error) fail(analysis.error); }, [analysis.error]); // eslint-disable-line react-hooks/exhaustive-deps
  const playable = item.kind === 'series' || !!item.stream_url;
  return (
    <>
      {open && (
        <DropdownMenuContent align="end" className="w-60">
          <MoreMenuItems
            item={item}
            admin={admin}
            playable={playable}
            onOpen={dialogs.open}
            onRefresh={() => refresh.mutate()}
            refreshing={refresh.isPending}
            onAnalyze={analysis.analyze}
            analyzing={analysis.analyzing}
            onQueue={(next) => void queue.add(next)}
            onPlaylist={() => setPlaylist(true)}
            extra={
              <>
                <DropdownMenuItem onSelect={() => played.mutate()}>
                  {watched ? <Circle className="size-4 text-muted" /> : <Check className="size-4 text-muted" />}
                  {watched ? t('titles:watchedButton.markUnwatched') : t('titles:watchedButton.markAsWatched')}
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => watchlist.mutate(!listed)}>
                  {listed ? <BookmarkCheck className="size-4 text-muted" /> : <Bookmark className="size-4 text-muted" />}
                  {listed ? t('titles:cardMenu.removeFromWatchlist') : t('titles:rating.addWatchlist')}
                </DropdownMenuItem>
              </>
            }
          />
        </DropdownMenuContent>
      )}
      {dialogs.dialogs}
      <AddToPlaylistDialog item={item} open={playlist} onOpenChange={setPlaylist} />
    </>
  );
}
