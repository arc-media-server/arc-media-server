import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, ListMusic, ListVideo, Loader2, Plus } from 'lucide-react';
import { api, type Item, type Playlist } from '@/lib/api';
import { cn } from '@/lib/utils';
import { ErrorNotice } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

/** A playlist's picture: its first four posters, or one, or an icon. */
export function PlaylistCover({ playlist, className }: { playlist: Playlist; className?: string }) {
  const images = playlist.images ?? [];
  return (
    <div className={cn('relative aspect-square overflow-hidden rounded-xl bg-gradient-to-br from-surface-2 to-surface-3 ring-1 ring-border', className)}>
      {images.length >= 4 ? (
        <div className="grid size-full grid-cols-2 grid-rows-2">
          {images.slice(0, 4).map((src) => (
            <img key={src} src={src} alt="" className="size-full object-cover" loading="lazy" />
          ))}
        </div>
      ) : images.length > 0 ? (
        <img src={images[0]} alt="" className="size-full object-cover" loading="lazy" />
      ) : (
        <div className="flex size-full items-center justify-center text-subtle">
          <ListVideo className="size-10" strokeWidth={1.5} />
        </div>
      )}
    </div>
  );
}

/** An "Add to Playlist" button that opens the picker. */
export function AddToPlaylistButton({ item, label, className, size = 'lg' }: { item: Item; label?: boolean; className?: string; size?: 'sm' | 'lg' | 'icon' }) {
  const { t } = useTranslation('playlists');
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        variant="secondary"
        size={size}
        className={cn(!label && size === 'lg' && 'size-12 rounded-2xl px-0', className)}
        title={t('picker.hint')}
        aria-label={t('picker.title')}
        onClick={(e) => {
          e.stopPropagation();
          setOpen(true);
        }}
      >
        <ListMusic /> {label && t('picker.title')}
      </Button>
      {/* React events bubble out of the dialog's portal: keep them from the row it sits in (an episode row plays on click). */}
      <span className="contents" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
        <AddToPlaylistDialog item={item} open={open} onOpenChange={setOpen} />
      </span>
    </>
  );
}

export function AddToPlaylistDialog({ item, open, onOpenChange }: { item: Item; open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <AddToPlaylist item={item} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function AddToPlaylist({ item, onDone }: { item: Item; onDone: () => void }) {
  const { t } = useTranslation('playlists');
  const qc = useQueryClient();
  const playlists = useQuery({ queryKey: ['playlists'], queryFn: api.playlists });
  const editable = playlists.data?.filter((p) => !p.smart_filter) ?? [];
  const [name, setName] = useState('');
  const [added, setAdded] = useState<string | null>(null);
  const done = (id: string) => {
    qc.invalidateQueries({ queryKey: ['playlists'] });
    qc.invalidateQueries({ queryKey: ['playlist'] });
    setAdded(id);
    window.setTimeout(onDone, 700);
  };
  const add = useMutation({ mutationFn: (id: string) => api.addToPlaylist(id, [item.id]), onSuccess: (_, id) => done(id) });
  const create = useMutation({ mutationFn: () => api.createPlaylist(name.trim(), [item.id]), onSuccess: (p) => done(p.id) });
  const error = add.error ?? create.error;
  const busy = add.isPending || create.isPending || added !== null;
  const canCreate = !!playlists.data && !playlists.error;


  return (
    <DialogContent title={t('picker.title')} description={item.kind === 'series' ? t('picker.addSeriesDescription', { title: item.title }) : t('picker.addDescription', { title: item.title })} className="w-[min(480px,calc(100vw-32px))]">
      <div className="space-y-4" onClick={(e) => e.stopPropagation()}>
        {error && <ErrorNotice error={error} />}
        {playlists.error && <div className="space-y-2">
          {playlists.data && <p className="text-xs text-muted">{t('picker.stale')}</p>}
          <ErrorNotice error={playlists.error} onRetry={() => void playlists.refetch()} busy={playlists.isFetching} />
        </div>}
        {!playlists.data && !playlists.error && <p role="status" className="text-sm text-muted">{t('picker.loading')}</p>}
        {playlists.data && !playlists.error && editable.length === 0 && <p className="text-sm text-muted">{t('picker.empty')}</p>}
        {!canCreate && <p className="text-xs text-muted">{playlists.error ? t('picker.refreshFirst') : t('picker.loadFirst')}</p>}
        {editable.length > 0 && (
          <div className="max-h-72 divide-y divide-border overflow-y-auto rounded-2xl ring-1 ring-border">
            {editable.map((p) => (
              <button
                key={p.id}
                type="button"
                disabled={busy}
                onClick={() => add.mutate(p.id)}
                className="flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-surface-2 disabled:opacity-70"
              >
                <PlaylistCover playlist={p} className="w-10 rounded-lg" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium">{p.title}</span>
                  <span className="block text-xs text-subtle">{t('titleCount', { count: p.count })}</span>
                </span>
                {added === p.id ? <Check className="size-4 text-accent" /> : add.isPending && add.variables === p.id && <Loader2 className="size-4 animate-spin" />}
              </button>
            ))}
          </div>
        )}
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (canCreate && !busy && name.trim()) create.mutate();
          }}
        >
          <input autoFocus={!editable.length} value={name} onChange={(e) => setName(e.target.value)} placeholder={t('picker.new')} aria-label={t('picker.new')} maxLength={100} className={inputClass} />
          <Button type="submit" variant="secondary" disabled={!canCreate || !name.trim() || busy}>
            {create.isPending ? <Loader2 className="animate-spin" /> : <Plus />}{' '}{t('common:actions.create')}</Button>
        </form>
      </div>
    </DialogContent>
  );
}
