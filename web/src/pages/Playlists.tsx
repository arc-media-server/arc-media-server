import { useTranslation } from 'react-i18next';
import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Reorder, useDragControls } from 'motion/react';
import { ArrowUp, ArrowDown, GripVertical, ListVideo, Loader2, Pencil, Play, Plus, Shuffle, Trash2, Wand2, X } from 'lucide-react';
import { shuffled } from '@/lib/shuffle';
import { api, type Playlist, type PlaylistEntry, type RowFilter } from '@/lib/api';
import { FilterFields, FilterPreview } from '@/components/FilterFields';
import { useLibraries } from '@/components/Sidebar';
import * as fmt from '@/lib/format';
import { Artwork } from '@/components/Artwork';
import { EmptyState, ErrorState, ErrorNotice, Page, PageHeader } from '@/components/Page';
import { nowPlayingFor, usePlayer } from '@/components/Player';
import { PlaylistCover } from '@/components/Playlists';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

/** The profile's playlists. */
export function PlaylistsPage() {
  const { t } = useTranslation('playlists');
  const navigate = useNavigate();
  const { data, error } = useQuery({ queryKey: ['playlists'], queryFn: api.playlists });
  const [naming, setNaming] = useState(false);
  const [smart, setSmart] = useState(false);
  if (error) return <ErrorState error={error} />;
  return (
    <Page className="pb-16">
      <PageHeader
        title={t('common:navigation.playlists')}
        subtitle={t('description')}
        actions={
          <>
            <Button variant="ghost" size="sm" onClick={() => setSmart(true)} title={t('smart.hint')}>
              <Wand2 />{' '}{t('smart.new')}</Button>
            <Button variant="secondary" size="sm" onClick={() => setNaming(true)}>
              <Plus />{' '}{t('new')}</Button>
          </>
        }
      />
      <div className="px-6 md:px-10">
        {!data && <div className="h-48 animate-pulse rounded-2xl bg-surface" />}
        {data?.length === 0 && (
          <EmptyState icon={<ListVideo />} title={t('empty.heading')}>{t('empty.description')}</EmptyState>
        )}
        <div className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-x-5 gap-y-7 sm:grid-cols-[repeat(auto-fill,minmax(190px,1fr))]">
          {data?.map((p) => (
            <Link key={p.id} to={`/playlist/${p.id}`} className="group block outline-none">
              <PlaylistCover playlist={p} className="shadow-lg shadow-black/25 transition-transform duration-300 group-hover:-translate-y-1" />
              <p className="mt-2.5 flex items-center gap-1 truncate text-[13px] font-medium">
                {p.smart_filter && <Wand2 className="size-3 shrink-0 text-accent" />}
                <span className="truncate">{p.title}</span>
              </p>
              <p className="text-xs text-subtle">
                {t('titleCount', { count: p.count })}
                {p.duration_ms > 0 && ` · ${fmt.runtime(Math.round(p.duration_ms / 60000))}`}
              </p>
            </Link>
          ))}
        </div>
      </div>
      <NameDialog open={naming} onOpenChange={setNaming} title={t('new')} onSave={async (name) => navigate(`/playlist/${(await api.createPlaylist(name)).id}`)} />
      <SmartPlaylistDialog open={smart} onOpenChange={setSmart} onSaved={(id) => navigate(`/playlist/${id}`)} />
    </Page>
  );
}

/** One playlist: play it, shuffle it, arrange it. */
export function PlaylistPage() {
  const { t } = useTranslation('playlists');
  const { id } = useParams();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const play = usePlayer();
  const { data, error } = useQuery({ queryKey: ['playlist', id], queryFn: () => api.playlist(id!) });
  const [renaming, setRenaming] = useState(false);
  const [editingRules, setEditingRules] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const remove = useMutation({
    mutationFn: () => api.deletePlaylist(id!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['playlists'] });
      navigate('/playlists', { replace: true });
    },
  });

  if (error) return <ErrorState error={error} />;
  if (!data) return <div className="h-64 animate-pulse bg-surface" />;
  const p = data.playlist;
  const queue = data.entries.map((e) => nowPlayingFor(e.item));
  const playFrom = (i: number, list = queue) => list.length && play(list[i], list);
  const shuffle = () => playFrom(0, shuffled(queue));

  return (
    <Page className="pb-16">
      <div className="flex flex-wrap items-end gap-6 px-6 pb-8 pt-14 md:px-10">
        <PlaylistCover playlist={{ ...p, images: data.entries.map((e) => (e.item.kind === 'episode' ? e.item.images?.series_primary : e.item.images?.primary)).filter((x): x is string => !!x).filter((x, i, a) => a.indexOf(x) === i) }} className="w-40 shadow-2xl" />
        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-accent">{p.smart_filter ? t('kind.smart') : t('kind.manual')}</p>
          <h1 className="mt-1 text-4xl font-bold leading-tight">{p.title}</h1>
          <p className="mt-1 text-sm text-muted">
            {t('titleCount', { count: p.count })}
            {p.duration_ms > 0 && ` · ${fmt.runtime(Math.round(p.duration_ms / 60000))}`}
          </p>
          <div className="mt-5 flex flex-wrap gap-3">
            <Button variant="primary" size="lg" disabled={!queue.length} onClick={() => playFrom(0)}>
              <Play className="fill-current" />{' '}{t('common:actions.play')}</Button>
            <Button variant="secondary" size="lg" disabled={queue.length < 2} onClick={shuffle}>
              <Shuffle />{' '}{t('common:actions.shuffle')}</Button>
            {p.smart_filter ? (
              <Button variant="ghost" size="lg" onClick={() => setEditingRules(true)}>
                <Wand2 />{' '}{t('smart.editRules')}</Button>
            ) : (
              <Button variant="ghost" size="lg" onClick={() => setRenaming(true)}>
                <Pencil />{' '}{t('common:actions.rename')}</Button>
            )}
            <Button variant="ghost" size="lg" onClick={() => setDeleting(true)}>
              <Trash2 />{' '}{t('common:actions.delete')}</Button>
          </div>
        </div>
      </div>
      <div className="px-6 md:px-10">
        {data.entries.length === 0 ? (
          <EmptyState icon={<ListVideo />} title={t('entries.emptyHeading')}>
            {p.smart_filter ? t('entries.smartEmpty') : t('entries.empty')}
          </EmptyState>
        ) : p.smart_filter ? (
          <div className="max-w-4xl space-y-1.5">
            {data.entries.map((e, i) => (
              <EntryRow key={e.item.id} entry={e} index={i} onPlay={() => playFrom(i)} />
            ))}
          </div>
        ) : (
          <Entries playlistId={p.id} entries={data.entries} onPlay={(i) => playFrom(i)} />
        )}
      </div>
      <NameDialog
        open={renaming}
        onOpenChange={setRenaming}
        title={t('rename')}
        initial={p.title}
        onSave={async (name) => {
          await api.renamePlaylist(p.id, name);
          qc.invalidateQueries({ queryKey: ['playlist', id] });
          qc.invalidateQueries({ queryKey: ['playlists'] });
        }}
      />
      {p.smart_filter && (
        <SmartPlaylistDialog
          open={editingRules}
          onOpenChange={setEditingRules}
          playlist={p}
          onSaved={() => {
            qc.invalidateQueries({ queryKey: ['playlist', id] });
            qc.invalidateQueries({ queryKey: ['playlists'] });
          }}
        />
      )}
      <ConfirmDialog open={deleting} onOpenChange={setDeleting} title={t('delete.heading', { title: p.title })} confirmLabel={t('common:actions.delete')} destructive busy={remove.isPending} error={remove.error?.message} onConfirm={() => remove.mutate()}>
        <p>{t('delete.description')}</p>
      </ConfirmDialog>
    </Page>
  );
}

function Entries({ playlistId, entries: initial, onPlay }: { playlistId: string; entries: PlaylistEntry[]; onPlay: (i: number) => void }) {
  const qc = useQueryClient();
  const [entries, setEntries] = useState(initial);
  useEffect(() => setEntries(initial), [initial]);
  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['playlist', playlistId] });
    qc.invalidateQueries({ queryKey: ['playlists'] });
  };
  const order = useMutation({ mutationFn: (ids: number[]) => api.orderPlaylist(playlistId, ids), onSuccess: refresh, onError: () => setEntries(initial) });
  const remove = useMutation({ mutationFn: (entryId: number) => api.removeFromPlaylist(playlistId, entryId), onSuccess: refresh });
  const move = (from: number, delta: number) => {
    const to = from + delta;
    if (order.isPending || to < 0 || to >= entries.length) return;
    const next = [...entries]; [next[from], next[to]] = [next[to], next[from]];
    setEntries(next); order.mutate(next.map(e => e.entry_id));
  };
  return (
    <>
    <ErrorNotice error={order.error || remove.error} onRetry={() => { if (order.error && order.variables) order.mutate(order.variables); else if (remove.variables !== undefined) remove.mutate(remove.variables); }} />
    <Reorder.Group axis="y" values={entries} onReorder={setEntries} className="max-w-4xl space-y-1.5">
      {entries.map((e, i) => (
        <EntryRow key={e.entry_id} entry={e} index={i} onPlay={() => onPlay(initial.findIndex((x) => x.entry_id === e.entry_id))} onMoveUp={i > 0 ? () => move(i, -1) : undefined} onMoveDown={i < entries.length - 1 ? () => move(i, 1) : undefined} busy={order.isPending} onDrop={() => { if (!order.isPending) order.mutate(entries.map((x) => x.entry_id)); }} onRemove={() => remove.mutate(e.entry_id)} />
      ))}
    </Reorder.Group>
    </>
  );
}

/** A playlist's title row; a smart playlist's can't be dragged or removed (its rules decide). */
function EntryRow({ entry, index, onPlay, onDrop, onRemove, onMoveUp, onMoveDown, busy }: { entry: PlaylistEntry; index: number; onPlay: () => void; onDrop?: () => void; onRemove?: () => void; onMoveUp?: () => void; onMoveDown?: () => void; busy?: boolean }) {
  const { t } = useTranslation('playlists');
  const drag = useDragControls();
  const it = entry.item;
  const episode = it.kind === 'episode';
  const fixed = !onDrop;
  const content = (
    <>
      {!fixed && (
        <button type="button" disabled={busy} aria-label={t('entryActions.drag', { title: it.title })} onPointerDown={(e) => drag.start(e)} className="cursor-grab touch-none p-1.5 text-subtle hover:text-foreground">
          <GripVertical className="size-4" />
        </button>
      )}
      {!fixed && <div className="flex shrink-0 flex-col"><button type="button" aria-label={t('entryActions.moveUp', { title: it.title })} disabled={!onMoveUp || busy} onClick={onMoveUp} className="rounded p-1 disabled:opacity-25"><ArrowUp className="size-4" /></button><button type="button" aria-label={t('entryActions.moveDown', { title: it.title })} disabled={!onMoveDown || busy} onClick={onMoveDown} className="rounded p-1 disabled:opacity-25"><ArrowDown className="size-4" /></button></div>}
      <span className="w-6 text-right text-sm tabular-nums text-subtle">{index + 1}</span>
      <button type="button" onClick={onPlay} className="relative hidden w-28 shrink-0 overflow-hidden rounded-lg sm:block" aria-label={t('entryActions.play', { title: it.title })}>
        <Artwork src={episode ? (it.images?.primary ?? it.images?.series_backdrop) : (it.images?.backdrop ?? it.images?.primary)} alt="" kind="still" icon={episode ? 'tv' : 'film'} fallbackTitle={it.title} />
        <span className="absolute inset-0 flex items-center justify-center bg-black/0 opacity-0 transition-all group-hover:bg-black/35 group-hover:opacity-100">
          <Play className="size-5 fill-white text-white" />
        </span>
      </button>
      <Link to={episode ? `/series/${it.parent_id}` : `/movie/${it.id}`} className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium hover:underline">{episode ? it.series_title : it.title}</p>
        <p className="truncate text-xs text-subtle">
          {episode ? [it.season_number !== undefined && t('entries.episodeCode', { season: it.season_number, episode: String(it.episode_number) }), it.title].filter(Boolean).join(' · ') : it.year}
          {it.runtime_min ? ` · ${fmt.runtime(it.runtime_min)}` : ''}
        </p>
      </Link>
      {onRemove && (
        <Button variant="ghost" size="icon" aria-label={t('entryActions.remove', { title: it.title })} onClick={onRemove}>
          <X />
        </Button>
      )}
    </>
  );
  const className = 'group flex items-center gap-3 rounded-2xl bg-surface p-2 pr-3 ring-1 ring-border';
  if (fixed) return <div className={className}>{content}</div>;
  return (
    <Reorder.Item value={entry} dragListener={false} dragControls={drag} onDragEnd={onDrop} className={className} whileDrag={{ scale: 1.01, boxShadow: '0 12px 30px rgba(0,0,0,0.35)' }}>
      {content}
    </Reorder.Item>
  );
}

/** Make a smart playlist, or change one's name and rules. */
function SmartPlaylistDialog({ open, onOpenChange, playlist, onSaved }: { open: boolean; onOpenChange: (o: boolean) => void; playlist?: Playlist; onSaved: (id: string) => void }) {
  return <Dialog open={open} onOpenChange={onOpenChange}>{open && <SmartPlaylistForm playlist={playlist} onDone={(id) => (onOpenChange(false), onSaved(id))} />}</Dialog>;
}

function SmartPlaylistForm({ playlist, onDone }: { playlist?: Playlist; onDone: (id: string) => void }) {
  const { t } = useTranslation('playlists');
  const libraries = (useLibraries().data ?? []).filter((l) => l.kind !== 'videos');
  const [title, setTitle] = useState(playlist?.title ?? '');
  const [filter, setFilter] = useState<RowFilter>(playlist?.smart_filter ?? { library_id: libraries[0]?.id ?? 0, unwatched: true, sort: 'shuffle' });
  useEffect(() => {
    if (!filter.library_id && libraries[0]) setFilter((f) => ({ ...f, library_id: libraries[0].id }));
  }, [filter.library_id, libraries]);
  const save = useMutation({
    mutationFn: async () => (playlist ? (await api.updateSmartPlaylist(playlist.id, title, filter), playlist.id) : (await api.createSmartPlaylist(title, filter)).id),
    onSuccess: onDone,
  });
  const library = libraries.find((l) => l.id === filter.library_id);
  return (
    <DialogContent
      title={playlist ? t('smart.edit') : t('smart.new')}
      description={t('smart.description')}
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost">{t('common:actions.cancel')}</Button>
          </DialogClose>
          <Button variant="primary" disabled={!title.trim() || !filter.library_id || save.isPending} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="animate-spin" />}{' '}{t('common:actions.save')}</Button>
        </>
      }
    >
      <div className="space-y-4 pb-2">
        <label className="block">
          <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t('common:fields.name')}</span>
          <input autoFocus value={title} onChange={(e) => setTitle(e.target.value)} maxLength={100} placeholder={t('smart.nameHint')} className={inputClass} />
        </label>
        <FilterFields filter={filter} onChange={setFilter} libraries={libraries} />
        <FilterPreview filter={filter} library={library} emptyText={t('smart.emptyPreview')} />
      </div>
    </DialogContent>
  );
}

function NameDialog({ open, onOpenChange, title, initial = '', onSave }: { open: boolean; onOpenChange: (o: boolean) => void; title: string; initial?: string; onSave: (name: string) => Promise<unknown> }) {
  const { t } = useTranslation('playlists');
  const [name, setName] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    if (open) setName(initial);
  }, [open, initial]);
  const save = async () => {
    setBusy(true);
    setError('');
    try {
      await onSave(name.trim());
      onOpenChange(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && (
        <DialogContent
          title={title}
          className="w-[min(440px,calc(100vw-32px))]"
          footer={
            <>
              {error && <p className="mr-auto text-sm text-warning">{error}</p>}
              <Button variant="ghost" onClick={() => onOpenChange(false)}>{t('common:actions.cancel')}</Button>
              <Button variant="primary" disabled={!name.trim() || busy} onClick={save}>{t('common:actions.save')}</Button>
            </>
          }
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (name.trim()) save();
            }}
          >
            <input autoFocus value={name} onChange={(e) => setName(e.target.value)} maxLength={100} placeholder={t('common:fields.name')} className={inputClass} />
          </form>
        </DialogContent>
      )}
    </Dialog>
  );
}
