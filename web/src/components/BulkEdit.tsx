import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { motion } from 'motion/react';
import { Gauge, Check, Merge, CircleCheck, Circle, Layers, Loader2, Pencil, RefreshCw, X } from 'lucide-react';
import { api, posterUrl, type BulkEdit, type Item, type Library } from '@/lib/api';
import { Artwork } from '@/components/Artwork';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';
import { ChipInput } from '@/components/ChipInput';
import { AddToCollectionDialog } from '@/components/Collections';
import { OptimizeDialog } from '@/components/Optimize';

/** The bar along the bottom while choosing titles in a library, with what can be done to them. */
export function SelectionBar({
  library,
  items,
  selected,
  onSelectAll,
  onClear,
  onDone,
}: {
  library: Library;
  items: Item[];
  selected: Set<string>;
  onSelectAll: () => void;
  onClear: () => void;
  onDone: () => void;
}) {
  const qc = useQueryClient();
  const chosen = items.filter((i) => selected.has(i.id));
  const [dialog, setDialog] = useState<'edit' | 'collection' | 'merge' | 'optimize' | null>(null);
  const [note, setNote] = useState('');
  const run = useMutation({
    mutationFn: (edit: Omit<BulkEdit, 'item_ids'>) => api.editItems({ item_ids: chosen.map((i) => i.id), ...edit }),
    onSuccess: (_, edit) => {
      for (const key of ['items', 'item', 'featured', 'continue', 'next-up', 'activity']) qc.invalidateQueries({ queryKey: [key] });
      setNote(edit.refresh ? 'Refreshing in the background. Follow it in Activity.' : '');
    },
  });
  const n = chosen.length;
  return (
    <>
      <motion.div
        initial={{ y: 80, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        className="fixed inset-x-0 bottom-5 z-40 flex justify-center px-4 md:left-[248px]"
      >
        <div className="flex max-w-full flex-wrap items-center gap-1.5 rounded-2xl border border-border bg-surface p-2 pl-4 shadow-2xl shadow-black/50">
          <span className="mr-1 text-sm font-medium tabular-nums">{n ? `${n} selected` : 'Choose titles'}</span>
          <Button variant="ghost" size="sm" onClick={n === items.length ? onClear : onSelectAll}>
            {n === items.length ? 'Select None' : `Select loaded (${items.length})`}
          </Button>
          <span className="mx-1 h-5 w-px bg-border" />
          <Button variant="secondary" size="sm" disabled={!n} onClick={() => setDialog('edit')}>
            <Pencil /> Edit
          </Button>
          <Button variant="ghost" size="sm" disabled={!n} onClick={() => setDialog('collection')}>
            <Layers /> Add to Collection
          </Button>
          {library.kind !== 'videos' && (
            <Button variant="ghost" size="sm" disabled={n < 2} onClick={() => setDialog('merge')} title={library.kind === 'shows' ? 'Make these one show' : 'Make these one movie, as versions'}>
              <Merge /> Merge
            </Button>
          )}
          <Button variant="ghost" size="sm" disabled={!n || run.isPending} onClick={() => run.mutate({ played: true })}>
            <CircleCheck /> Watched
          </Button>
          <Button variant="ghost" size="sm" disabled={!n || run.isPending} onClick={() => run.mutate({ played: false })}>
            <Circle /> Unwatched
          </Button>
          <Button variant="ghost" size="sm" disabled={!n} onClick={() => setDialog('optimize')}>
            <Gauge /> Optimize
          </Button>
          <Button variant="ghost" size="sm" disabled={!n || run.isPending} onClick={() => run.mutate({ refresh: true })}>
            <RefreshCw /> Refresh
          </Button>
          <span className="mx-1 h-5 w-px bg-border" />
          <Button variant="ghost" size="sm" onClick={onDone} aria-label="Stop choosing">
            <X /> Done
          </Button>
          {(run.error || note) && (
            <p className={cn('basis-full px-1 pb-1 text-xs', run.error ? 'text-warning' : 'text-subtle')}>{run.error?.message ?? note}</p>
          )}
        </div>
      </motion.div>
      <Dialog open={dialog === 'edit'} onOpenChange={(o) => setDialog(o ? 'edit' : null)}>
        {dialog === 'edit' && <BulkEditForm items={chosen} onDone={() => setDialog(null)} />}
      </Dialog>
      <Dialog open={dialog === 'merge'} onOpenChange={(o) => setDialog(o ? 'merge' : null)}>
        {dialog === 'merge' && (
          <MergeForm
            items={chosen}
            onDone={() => {
              setDialog(null);
              onDone();
            }}
          />
        )}
      </Dialog>
      <OptimizeDialog items={chosen} open={dialog === 'optimize'} onOpenChange={(o) => setDialog(o ? 'optimize' : null)} />
      <AddToCollectionDialog items={chosen} library={library} open={dialog === 'collection'} onOpenChange={(o) => setDialog(o ? 'collection' : null)} />
    </>
  );
}

/** Genres, labels, age rating and locks for several titles at once. */
function BulkEditForm({ items, onDone }: { items: Item[]; onDone: () => void }) {
  const qc = useQueryClient();
  const [addGenres, setAddGenres] = useState<string[]>([]);
  const [removeGenres, setRemoveGenres] = useState<string[]>([]);
  const [addLabels, setAddLabels] = useState<string[]>([]);
  const [removeLabels, setRemoveLabels] = useState<string[]>([]);
  const [rating, setRating] = useState('');
  const [locks, setLocks] = useState<'' | 'all' | 'none'>('');
  const labels = useQuery({ queryKey: ['labels'], queryFn: api.labels });
  const facets = useQuery({ queryKey: ['facets', items[0].library_id], queryFn: () => api.facets(items[0].library_id) });
  const genres = facets.data?.genres.map((g) => g.value);

  const save = useMutation({
    mutationFn: () =>
      api.editItems({
        item_ids: items.map((i) => i.id),
        add_genres: addGenres,
        remove_genres: removeGenres,
        add_labels: addLabels,
        remove_labels: removeLabels,
        content_rating: rating.trim() || undefined,
        locks: locks || undefined,
      }),
    onSuccess: () => {
      for (const key of ['items', 'item', 'facets', 'labels', 'featured']) qc.invalidateQueries({ queryKey: [key] });
      onDone();
    },
  });
  const changed = addGenres.length + removeGenres.length + addLabels.length + removeLabels.length > 0 || !!rating.trim() || !!locks;

  return (
    <DialogContent
      title={`Edit ${fmt.plural(items.length, 'title')}`}
      description="Changes apply to every title you chose. Anything you leave empty stays as it is."
      className="w-[min(560px,calc(100vw-32px))]"
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost">Cancel</Button>
          </DialogClose>
          <Button variant="primary" disabled={!changed || save.isPending} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="animate-spin" />} Save
          </Button>
        </>
      }
    >
      <div className="grid grid-cols-2 gap-x-4 gap-y-5 pb-2">
        <Field label="Add genres">
          <ChipInput value={addGenres} onChange={setAddGenres} placeholder="e.g. Thriller" morePlaceholder="Add another" suggestions={genres} />
        </Field>
        <Field label="Remove genres">
          <ChipInput value={removeGenres} onChange={setRemoveGenres} placeholder="e.g. Drama" morePlaceholder="Add another" suggestions={genres} />
        </Field>
        <Field label="Add labels">
          <ChipInput value={addLabels} onChange={setAddLabels} placeholder="e.g. Not for kids" morePlaceholder="Add another" suggestions={labels.data} />
        </Field>
        <Field label="Remove labels">
          <ChipInput value={removeLabels} onChange={setRemoveLabels} placeholder="e.g. Halloween" morePlaceholder="Add another" suggestions={labels.data} />
        </Field>
        <Field label="Age rating">
          <input value={rating} onChange={(e) => setRating(e.target.value)} placeholder="Leave as it is" maxLength={20} className={inputClass} />
        </Field>
        <div className="col-span-2">
          <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Locks</span>
          <div className="grid gap-2 sm:grid-cols-3">
            {(
              [
                ['', 'Leave as they are', 'Changed fields lock, as usual'],
                ['all', 'Lock everything', 'Refreshes change nothing'],
                ['none', 'Unlock everything', 'Use the online details again'],
              ] as const
            ).map(([v, label, hint]) => (
              <button
                key={v}
                type="button"
                aria-pressed={locks === v}
                onClick={() => setLocks(v)}
                className={cn(
                  'rounded-xl px-3 py-2.5 text-left ring-1 transition-colors',
                  locks === v ? 'bg-accent/10 ring-2 ring-accent' : 'bg-surface-2/50 ring-border hover:bg-surface-2',
                )}
              >
                <span className="flex items-center gap-1.5 text-sm font-medium">
                  {locks === v && <Check className="size-3.5 text-accent" />}
                  {label}
                </span>
                <span className="block text-xs text-subtle">{hint}</span>
              </button>
            ))}
          </div>
          {locks === 'none' && <p className="mt-2 text-xs text-subtle">Unlocked fields go back to the online details at the next refresh. Use Refresh to do it now.</p>}
        </div>
      </div>
    </DialogContent>
  );
}

/** Merge several movies into one (also offered for duplicates in Library Health). */
export function MergeDialog({ items, folders, open, onOpenChange }: { items: Item[]; folders?: Record<string, string>; open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <MergeForm items={items} folders={folders} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

/** Merge: choose which movie or show to keep; the others' files (and a show's episodes) join it. */
function MergeForm({ items, folders, onDone }: { items: Item[]; folders?: Record<string, string>; onDone: () => void }) {
  const qc = useQueryClient();
  // Keep a matched one by default, since its details are the ones that stay.
  const [keep, setKeep] = useState(() => (items.find((i) => i.metadata_state === 'matched') ?? items[0]).id);
  const shows = items[0]?.kind === 'series';
  const merge = useMutation({
    mutationFn: () => api.mergeItems(keep, items.map((i) => i.id)),
    onSuccess: () => {
      for (const key of ['items', 'item', 'seasons', 'featured', 'continue', 'collections', 'health', 'libraries']) qc.invalidateQueries({ queryKey: [key] });
      onDone();
    },
  });
  return (
    <DialogContent
      title={shows ? `Merge ${items.length} shows` : `Merge ${items.length} movies`}
      description={shows
        ? 'They become one show. Episodes they both have keep every file; the rest move across. Watch history, ratings, playlists and collections come along.'
        : 'They become one movie, with each file as a version you can choose when playing. Watch history, ratings, playlists and collections come along.'}
      className="w-[min(520px,calc(100vw-32px))]"
      footer={
        <>
          {merge.error && <p className="mr-auto text-sm text-warning">{merge.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost">Cancel</Button>
          </DialogClose>
          <Button variant="primary" disabled={merge.isPending} onClick={() => merge.mutate()}>
            {merge.isPending && <Loader2 className="animate-spin" />} Merge
          </Button>
        </>
      }
    >
      <span className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Keep the details of</span>
      <div className="max-h-[50vh] space-y-1.5 overflow-y-auto pb-2">
        {items.map((it) => (
          <button
            key={it.id}
            type="button"
            aria-pressed={keep === it.id}
            onClick={() => setKeep(it.id)}
            className={cn(
              'flex w-full items-center gap-3 rounded-xl p-2 pr-3 text-left ring-1 transition-colors',
              keep === it.id ? 'bg-accent/10 ring-2 ring-accent' : 'ring-border hover:bg-surface-2',
            )}
          >
            <div className="w-10 shrink-0 overflow-hidden rounded-md">
              <Artwork src={posterUrl(it)} alt="" fallbackTitle={it.title} icon="film" />
            </div>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{it.title}</span>
              <span className="block text-xs text-subtle">{[it.year, it.metadata_state !== 'matched' && 'not matched'].filter(Boolean).join(' · ')}</span>
              {folders?.[it.id] && <span className="block truncate font-mono text-[11px] text-subtle" title={folders[it.id]}>{folders[it.id]}</span>}
            </span>
            {keep === it.id && <Check className="size-4 text-accent" />}
          </button>
        ))}
      </div>
      <p className="text-xs text-subtle">
        {shows
          ? "The kept show's episode orders stay; the others' own orders and manual placements are dropped. Shows can't be split apart again."
          : 'Split a movie apart again from its ⋯ menu if you change your mind.'}
      </p>
    </DialogContent>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="col-span-2 sm:col-span-1">
      <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{label}</span>
      {children}
    </div>
  );
}
