import { useState } from 'react';
import { Link } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Filter, Layers, ListPlus, Loader2, Plus } from 'lucide-react';
import { api, type Collection, type Item, type Library, type RowFilter } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { Artwork } from '@/components/Artwork';
import { FilterFields, FilterPreview } from '@/components/FilterFields';
import { MediaRow } from '@/components/MediaRow';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Field, inputClass } from '@/components/ui/field';
import { ArtworkPanel, artworkNames, type ArtworkChange } from '@/components/ChooseArtwork';

export function CollectionCard({ collection: c, unit }: { collection: Collection; unit: string }) {
  return (
    <Link to={`/collection/${c.id}`} className="group block outline-none" draggable={false}>
      <div className="relative">
        {/* A second sleeve behind the poster says "several". */}
        <div className="absolute inset-x-2 -top-1.5 h-full rounded-xl bg-surface-3 ring-1 ring-border transition-transform duration-300 group-hover:-translate-y-0.5" />
        <div className="relative overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl group-focus-visible:ring-2 group-focus-visible:ring-accent">
          <Artwork src={c.images.primary} alt={c.title} fallbackTitle={c.title} />
        </div>
      </div>
      <div className="mt-2.5 px-0.5">
        <p className="flex items-center gap-1 truncate text-[13px] font-medium leading-tight">
          {c.kind === 'smart' && <Filter className="size-3 shrink-0 text-accent" />}
          <span className="truncate">{c.title}</span>
        </p>
        <p className="mt-0.5 text-xs text-subtle">{fmt.plural(c.count, unit)}</p>
      </div>
    </Link>
  );
}

export function CollectionGrid({ collections, unit }: { collections: Collection[]; unit: string }) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-8 pt-1.5 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
      {collections.map((c) => (
        <CollectionCard key={c.id} collection={c} unit={unit} />
      ))}
    </div>
  );
}

/** Make a manual or smart collection, or edit one. */
export function CollectionDialog({
  library,
  collection,
  open,
  onOpenChange,
  onSaved,
}: {
  library: Library;
  collection?: Collection;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved?: (c: Collection) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && (
        <CollectionEditor
          library={library}
          collection={collection}
          onDone={(c) => {
            onOpenChange(false);
            if (c) onSaved?.(c);
          }}
        />
      )}
    </Dialog>
  );
}

function CollectionEditor({ library, collection, onDone }: { library: Library; collection?: Collection; onDone: (c?: Collection) => void }) {
  const qc = useQueryClient();
  const [kind, setKind] = useState<'manual' | 'smart'>(collection?.kind === 'smart' ? 'smart' : collection ? 'manual' : 'manual');
  const [title, setTitle] = useState(collection?.title ?? '');
  const [sortTitle, setSortTitle] = useState(collection?.locked_fields.includes('sort_title') ? collection.sort_title : '');
  const [overview, setOverview] = useState(collection?.overview ?? '');
  // Editing: Details, or its poster or background (chosen images wait for Save).
  const [tab, setTab] = useState<'details' | 'poster' | 'backdrop'>('details');
  const [art, setArt] = useState<Partial<Record<'poster' | 'backdrop', ArtworkChange>>>({});
  const [filter, setFilter] = useState<RowFilter>(collection?.smart_filter ?? { library_id: library.id, sort: 'added' });
  const editing = !!collection;
  const auto = collection?.kind === 'auto';
  const save = useMutation({
    mutationFn: async () => {
      if (!editing) return api.createCollection({ library_id: library.id, kind, title, overview, filter: kind === 'smart' ? filter : undefined });
      const c = await api.updateCollection(collection!.id, { title, sort_title: sortTitle, overview, filter: kind === 'smart' ? filter : undefined });
      for (const [type, change] of Object.entries(art) as ['poster' | 'backdrop', ArtworkChange][]) {
        if ('file' in change) await api.uploadCollectionArtwork(collection!.id, type, change.file);
        else await api.chooseCollectionArtwork(collection!.id, type, 'path' in change ? change.path : null);
      }
      return c;
    },
    onSuccess: (c) => {
      for (const key of ['collections', 'collection', 'artwork']) qc.invalidateQueries({ queryKey: [key] });
      onDone(c);
    },
  });
  const unit = library.kind === 'shows' ? 'show' : 'movie';

  return (
    <DialogContent
      title={editing ? 'Edit Collection' : 'New Collection'}
      className={tab === 'details' ? 'w-[min(600px,calc(100vw-32px))]' : 'w-[min(820px,calc(100vw-32px))]'}
      description={tab !== 'details' ? undefined : 
        kind === 'smart'
          ? `A smart collection holds every ${unit} that matches, and keeps itself up to date.`
          : auto
            ? 'This collection comes from TMDB. You can rename it and change its description.'
            : `Add ${unit}s to it from their pages (⋯ > Add to Collection), in any order.`
      }
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <Button variant="ghost" onClick={() => onDone()}>
            Cancel
          </Button>
          <Button variant="primary" disabled={!title.trim() || save.isPending} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="animate-spin" />} {editing ? 'Save' : 'Create'}
          </Button>
        </>
      }
    >
      {editing && (
        <div className="mb-5 flex gap-1" role="tablist">
          {(['details', 'poster', 'backdrop'] as const).map((t) => (
            <button
              key={t}
              type="button"
              role="tab"
              aria-selected={tab === t}
              onClick={() => setTab(t)}
              className={cn('rounded-full px-3.5 py-1 text-[13px] font-medium transition-colors', tab === t ? 'bg-foreground text-background' : 'text-muted hover:text-foreground')}
            >
              {t === 'details' ? 'Details' : artworkNames[t]}
              {t !== 'details' && art[t] && <span className="ml-1.5 inline-block size-1.5 rounded-full bg-accent align-middle" aria-label="changed" />}
            </button>
          ))}
        </div>
      )}
      {editing && tab !== 'details' && (
        <ArtworkPanel
          item={{ id: collection!.id, kind: 'movie' }}
          type={tab}
          change={art[tab]}
          onChange={(c) => setArt((prev) => ({ ...prev, [tab]: c }))}
          source={{ key: ['artwork', 'collection', collection!.id], load: () => api.collectionArtwork(collection!.id) }}
        />
      )}
      <div className={cn('space-y-4', tab !== 'details' && 'hidden')}>
        {!editing && (
          <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label="Kind">
            {(
              [
                ['manual', ListPlus, 'Choose the titles', 'Pick them yourself'],
                ['smart', Filter, 'Smart', 'Everything that matches a filter'],
              ] as const
            ).map(([k, Icon, label, hint]) => (
              <button
                key={k}
                type="button"
                role="radio"
                aria-checked={kind === k}
                onClick={() => setKind(k)}
                className={cn(
                  'flex items-start gap-3 rounded-2xl p-3 text-left ring-1 transition-colors',
                  kind === k ? 'bg-accent/10 ring-2 ring-accent' : 'bg-surface-2/60 ring-border hover:bg-surface-2',
                )}
              >
                <Icon className="mt-0.5 size-4 text-accent" />
                <span>
                  <span className="block text-sm font-medium">{label}</span>
                  <span className="block text-xs text-subtle">{hint}</span>
                </span>
              </button>
            ))}
          </div>
        )}
        <Field label="Name">
          <input autoFocus value={title} onChange={(e) => setTitle(e.target.value)} maxLength={100} placeholder="e.g. Christmas Favourites" className={inputClass} />
        </Field>
        {editing && (
          <Field label="Sort As">
            <input value={sortTitle} onChange={(e) => setSortTitle(e.target.value)} maxLength={100} placeholder={title || 'Same as the name'} className={inputClass} />
          </Field>
        )}
        <Field label="Description">
          <textarea value={overview} onChange={(e) => setOverview(e.target.value)} rows={2} className={cn(inputClass, 'h-auto resize-y py-2.5')} />
        </Field>
        {kind === 'smart' && (
          <>
            <FilterFields filter={filter} onChange={setFilter} unwatchedLabel="Only what the person watching hasn't seen" />
            <FilterPreview filter={filter} library={library} emptyText="Nothing matches yet. It'll fill up as titles are added." />
          </>
        )}
      </div>
    </DialogContent>
  );
}

/** Put a title in one of its library's manual collections, or a new one. */
export function AddToCollectionDialog({ item, library, open, onOpenChange }: { item: Item; library?: Library; open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && library && <AddToCollection item={item} library={library} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function AddToCollection({ item, library, onDone }: { item: Item; library: Library; onDone: () => void }) {
  const qc = useQueryClient();
  const collections = useQuery({ queryKey: ['collections', library.id], queryFn: () => api.collections(library.id) });
  const [name, setName] = useState('');
  const inIt = new Set(item.collections?.map((c) => c.id));
  const done = () => {
    for (const key of ['collections', 'collection', 'item']) qc.invalidateQueries({ queryKey: [key] });
    onDone();
  };
  const add = useMutation({ mutationFn: (id: string) => api.addToCollection(id, [item.id]), onSuccess: done });
  const create = useMutation({
    mutationFn: () => api.createCollection({ library_id: library.id, kind: 'manual', title: name.trim(), item_ids: [item.id] }),
    onSuccess: done,
  });
  const manual = collections.data?.filter((c) => c.kind === 'manual') ?? [];
  const error = add.error ?? create.error;

  return (
    <DialogContent title="Add to Collection" description={`Put ${item.title} in a collection.`} className="w-[min(480px,calc(100vw-32px))]">
      <div className="space-y-4">
        {error && <p className="text-sm text-warning">{error.message}</p>}
        {manual.length > 0 && (
          <div className="divide-y divide-border overflow-hidden rounded-2xl ring-1 ring-border">
            {manual.map((c) => (
              <button
                key={c.id}
                type="button"
                disabled={inIt.has(c.id) || add.isPending}
                onClick={() => add.mutate(c.id)}
                className="flex w-full items-center gap-3 px-4 py-3 text-left text-sm transition-colors hover:bg-surface-2 disabled:opacity-60"
              >
                <Layers className="size-4 text-muted" />
                <span className="flex-1 truncate">{c.title}</span>
                {inIt.has(c.id) ? <Check className="size-4 text-accent" /> : <span className="text-xs text-subtle">{fmt.plural(c.count, 'title')}</span>}
              </button>
            ))}
          </div>
        )}
        <form
          className="flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) create.mutate();
          }}
        >
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="New collection" maxLength={100} className={inputClass} />
          <Button type="submit" variant="secondary" disabled={!name.trim() || create.isPending}>
            {create.isPending ? <Loader2 className="animate-spin" /> : <Plus />} Create
          </Button>
        </form>
      </div>
    </DialogContent>
  );
}

/** On a title's page: a row for each collection it's in, with the rest of the collection. */
export function CollectionRows({ item }: { item: Item }) {
  return (
    <>
      {item.collections?.map((c) => (
        <CollectionRow key={c.id} id={c.id} title={c.title} exclude={item.id} />
      ))}
    </>
  );
}

function CollectionRow({ id, title, exclude }: { id: string; title: string; exclude: string }) {
  const { data } = useQuery({ queryKey: ['collection', id], queryFn: () => api.collection(id) });
  const items = data?.items.filter((i) => i.id !== exclude) ?? [];
  return (
    <div className="mt-10">
      <MediaRow title={title} items={items} href={`/collection/${id}`} />
    </div>
  );
}
