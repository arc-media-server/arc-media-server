import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Reorder, useDragControls } from 'motion/react';
import { GripVertical, ListVideo, Pencil, Play, Plus, Shuffle, Trash2, X } from 'lucide-react';
import { shuffled } from '@/lib/shuffle';
import { api, type PlaylistEntry } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Artwork } from '@/components/Artwork';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { nowPlayingFor, usePlayer } from '@/components/Player';
import { PlaylistCover } from '@/components/Playlists';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

/** The profile's playlists. */
export function PlaylistsPage() {
  const navigate = useNavigate();
  const { data, error } = useQuery({ queryKey: ['playlists'], queryFn: api.playlists });
  const [naming, setNaming] = useState(false);
  if (error) return <ErrorState error={error} />;
  return (
    <Page className="pb-16">
      <PageHeader
        title="Playlists"
        subtitle="Movies and episodes to watch in order. Just for your profile."
        actions={
          <Button variant="secondary" size="sm" onClick={() => setNaming(true)}>
            <Plus /> New Playlist
          </Button>
        }
      />
      <div className="px-6 md:px-10">
        {!data && <div className="h-48 animate-pulse rounded-2xl bg-surface" />}
        {data?.length === 0 && (
          <EmptyState icon={<ListVideo />} title="No playlists yet">
            Add a movie, an episode or a whole show to one from its page, or make an empty one with New Playlist.
          </EmptyState>
        )}
        <div className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-x-5 gap-y-7 sm:grid-cols-[repeat(auto-fill,minmax(190px,1fr))]">
          {data?.map((p) => (
            <Link key={p.id} to={`/playlist/${p.id}`} className="group block outline-none">
              <PlaylistCover playlist={p} className="shadow-lg shadow-black/25 transition-transform duration-300 group-hover:-translate-y-1" />
              <p className="mt-2.5 truncate text-[13px] font-medium">{p.title}</p>
              <p className="text-xs text-subtle">
                {fmt.plural(p.count, 'title')}
                {p.duration_ms > 0 && ` · ${fmt.runtime(Math.round(p.duration_ms / 60000))}`}
              </p>
            </Link>
          ))}
        </div>
      </div>
      <NameDialog open={naming} onOpenChange={setNaming} title="New Playlist" onSave={async (name) => navigate(`/playlist/${(await api.createPlaylist(name)).id}`)} />
    </Page>
  );
}

/** One playlist: play it, shuffle it, arrange it. */
export function PlaylistPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const play = usePlayer();
  const { data, error } = useQuery({ queryKey: ['playlist', id], queryFn: () => api.playlist(id!) });
  const [renaming, setRenaming] = useState(false);
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
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-accent">Playlist</p>
          <h1 className="mt-1 text-4xl font-bold leading-tight">{p.title}</h1>
          <p className="mt-1 text-sm text-muted">
            {fmt.plural(p.count, 'title')}
            {p.duration_ms > 0 && ` · ${fmt.runtime(Math.round(p.duration_ms / 60000))}`}
          </p>
          <div className="mt-5 flex flex-wrap gap-3">
            <Button variant="primary" size="lg" disabled={!queue.length} onClick={() => playFrom(0)}>
              <Play className="fill-current" /> Play
            </Button>
            <Button variant="secondary" size="lg" disabled={queue.length < 2} onClick={shuffle}>
              <Shuffle /> Shuffle
            </Button>
            <Button variant="ghost" size="lg" onClick={() => setRenaming(true)}>
              <Pencil /> Rename
            </Button>
            <Button variant="ghost" size="lg" onClick={() => setDeleting(true)}>
              <Trash2 /> Delete
            </Button>
          </div>
        </div>
      </div>
      <div className="px-6 md:px-10">
        {data.entries.length === 0 ? (
          <EmptyState icon={<ListVideo />} title="Nothing in it yet">
            Add movies, episodes or whole shows from their pages.
          </EmptyState>
        ) : (
          <Entries playlistId={p.id} entries={data.entries} onPlay={(i) => playFrom(i)} />
        )}
      </div>
      <NameDialog
        open={renaming}
        onOpenChange={setRenaming}
        title="Rename Playlist"
        initial={p.title}
        onSave={async (name) => {
          await api.renamePlaylist(p.id, name);
          qc.invalidateQueries({ queryKey: ['playlist', id] });
          qc.invalidateQueries({ queryKey: ['playlists'] });
        }}
      />
      <ConfirmDialog open={deleting} onOpenChange={setDeleting} title={`Delete ${p.title}?`} confirmLabel="Delete" destructive busy={remove.isPending} error={remove.error?.message} onConfirm={() => remove.mutate()}>
        <p>The playlist goes; its movies and episodes stay in your libraries.</p>
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
  const order = useMutation({ mutationFn: (ids: number[]) => api.orderPlaylist(playlistId, ids), onSuccess: refresh });
  const remove = useMutation({ mutationFn: (entryId: number) => api.removeFromPlaylist(playlistId, entryId), onSuccess: refresh });
  return (
    <Reorder.Group axis="y" values={entries} onReorder={setEntries} className="max-w-4xl space-y-1.5">
      {entries.map((e, i) => (
        <EntryRow key={e.entry_id} entry={e} index={i} onPlay={() => onPlay(initial.findIndex((x) => x.entry_id === e.entry_id))} onDrop={() => order.mutate(entries.map((x) => x.entry_id))} onRemove={() => remove.mutate(e.entry_id)} />
      ))}
    </Reorder.Group>
  );
}

function EntryRow({ entry, index, onPlay, onDrop, onRemove }: { entry: PlaylistEntry; index: number; onPlay: () => void; onDrop: () => void; onRemove: () => void }) {
  const drag = useDragControls();
  const it = entry.item;
  const episode = it.kind === 'episode';
  return (
    <Reorder.Item
      value={entry}
      dragListener={false}
      dragControls={drag}
      onDragEnd={onDrop}
      className="group flex items-center gap-3 rounded-2xl bg-surface p-2 pr-3 ring-1 ring-border"
      whileDrag={{ scale: 1.01, boxShadow: '0 12px 30px rgba(0,0,0,0.35)' }}
    >
      <button type="button" aria-label={`Drag ${it.title}`} onPointerDown={(e) => drag.start(e)} className="cursor-grab touch-none p-1.5 text-subtle hover:text-foreground">
        <GripVertical className="size-4" />
      </button>
      <span className="w-6 text-right text-sm tabular-nums text-subtle">{index + 1}</span>
      <button type="button" onClick={onPlay} className="relative w-28 shrink-0 overflow-hidden rounded-lg" aria-label={`Play ${it.title}`}>
        <Artwork src={episode ? (it.images?.primary ?? it.images?.series_backdrop) : (it.images?.backdrop ?? it.images?.primary)} alt="" kind="still" icon={episode ? 'tv' : 'film'} fallbackTitle={it.title} />
        <span className="absolute inset-0 flex items-center justify-center bg-black/0 opacity-0 transition-all group-hover:bg-black/35 group-hover:opacity-100">
          <Play className="size-5 fill-white text-white" />
        </span>
      </button>
      <Link to={episode ? `/series/${it.parent_id}` : `/movie/${it.id}`} className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium hover:underline">{episode ? it.series_title : it.title}</p>
        <p className="truncate text-xs text-subtle">
          {episode ? [it.season_number !== undefined && `S${it.season_number} E${it.episode_number}`, it.title].filter(Boolean).join(' · ') : it.year}
          {it.runtime_min ? ` · ${fmt.runtime(it.runtime_min)}` : ''}
        </p>
      </Link>
      <Button variant="ghost" size="icon" aria-label={`Remove ${it.title}`} onClick={onRemove}>
        <X />
      </Button>
    </Reorder.Item>
  );
}

function NameDialog({ open, onOpenChange, title, initial = '', onSave }: { open: boolean; onOpenChange: (o: boolean) => void; title: string; initial?: string; onSave: (name: string) => Promise<unknown> }) {
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
              <Button variant="ghost" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button variant="primary" disabled={!name.trim() || busy} onClick={save}>
                Save
              </Button>
            </>
          }
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (name.trim()) save();
            }}
          >
            <input autoFocus value={name} onChange={(e) => setName(e.target.value)} maxLength={100} placeholder="Name" className={inputClass} />
          </form>
        </DialogContent>
      )}
    </Dialog>
  );
}
