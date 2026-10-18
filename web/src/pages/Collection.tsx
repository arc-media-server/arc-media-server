import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Reorder, useDragControls } from 'motion/react';
import { ArrowUpDown, Check, GripVertical, Pencil, Trash2, X } from 'lucide-react';
import { api, type Item } from '@/lib/api';
import { kindUnit } from '@/lib/kinds';
import { useIsAdmin } from '@/lib/auth';
import * as fmt from '@/lib/format';
import { Artwork } from '@/components/Artwork';
import { CollectionDialog } from '@/components/Collections';
import { Hero } from '@/components/Hero';
import { ErrorState, Page } from '@/components/Page';
import { PosterGrid } from '@/components/PosterCard';
import { useLibraries } from '@/components/Sidebar';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

const kindLabels = { auto: 'Collection', manual: 'Collection', smart: 'Smart Collection', folder: 'Folder' };

export function CollectionPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const admin = useIsAdmin();
  const { data, error } = useQuery({ queryKey: ['collection', id], queryFn: () => api.collection(id!) });
  const { data: libraries } = useLibraries();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [arranging, setArranging] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.deleteCollection(id!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['collections'] });
      navigate(`/library/${data?.collection.library_id}?view=collections`, { replace: true });
    },
  });

  if (error) return <ErrorState error={error} />;
  if (!data) return <div className="h-[60vh] animate-pulse bg-surface" />;
  const c = data.collection;
  const library = libraries?.find((l) => l.id === c.library_id);
  const unit = kindUnit(library?.kind)[0];

  return (
    <Page className="pb-20">
      <Hero backdrop={c.images.backdrop} className="min-h-[52vh]">
        <div className="flex min-h-[52vh] items-end gap-8 px-6 pb-10 pt-24 md:px-10">
          <Artwork src={c.images.primary} alt={c.title} fallbackTitle={c.title} eager className="hidden w-44 shrink-0 rounded-2xl shadow-2xl shadow-black/50 ring-1 ring-white/10 lg:block" />
          <div className="max-w-2xl">
            <p className="mb-2 text-xs font-semibold uppercase tracking-[0.18em] text-accent">{kindLabels[c.kind]}</p>
            <h1 className="text-4xl font-bold leading-[1.05] md:text-5xl">{c.title}</h1>
            <p className="mt-3 text-sm text-foreground/70">{fmt.plural(c.count, unit)}</p>
            {c.overview && <p className="mt-4 text-[15px] leading-relaxed text-foreground/85">{c.overview}</p>}
            {admin && (
              <div className="mt-6 flex flex-wrap gap-3">
                <Button variant="secondary" onClick={() => setEditing(true)}>
                  <Pencil /> Edit
                </Button>
                {c.kind === 'manual' && data.items.length > 1 && (
                  <Button variant="secondary" onClick={() => setArranging((a) => !a)}>
                    {arranging ? <Check /> : <ArrowUpDown />} {arranging ? 'Done' : 'Arrange'}
                  </Button>
                )}
                <Button variant="ghost" onClick={() => setDeleting(true)}>
                  <Trash2 /> {c.kind === 'auto' || c.kind === 'folder' ? 'Remove' : 'Delete'}
                </Button>
              </div>
            )}
          </div>
        </div>
      </Hero>

      <div className="px-6 pt-8 md:px-10">
        {arranging ? <Arrange collectionId={c.id} items={data.items} /> : data.items.length > 0 && <PosterGrid items={data.items} landscape={c.kind === 'folder'} />}
        {!arranging && data.items.length === 0 && (
          <p className="text-sm text-muted">{c.kind === 'manual' ? `Nothing in it yet. Add ${unit}s from their pages (⋯ > Add to Collection).` : 'Nothing matches right now.'}</p>
        )}
      </div>

      {library && <CollectionDialog library={library} collection={c} open={editing} onOpenChange={setEditing} />}
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={c.kind === 'auto' ? `Remove ${c.title}?` : `Delete ${c.title}?`}
        confirmLabel={c.kind === 'auto' ? 'Remove' : 'Delete'}
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => remove.mutate()}
      >
        <p>
          {c.kind === 'auto'
            ? "It won't be shown again, even though TMDB groups these films. The films themselves stay in your library."
            : 'The collection goes; the titles in it stay in your library.'}
        </p>
      </ConfirmDialog>
    </Page>
  );
}

/** Drag a manual collection's titles into order, or take them out. */
function Arrange({ collectionId, items: initial }: { collectionId: string; items: Item[] }) {
  const qc = useQueryClient();
  const [items, setItems] = useState(initial);
  useEffect(() => setItems(initial), [initial]);
  const save = useMutation({
    mutationFn: (ids: string[]) => api.orderCollection(collectionId, ids),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['collection', collectionId] }),
  });
  const removeItem = useMutation({
    mutationFn: (itemId: string) => api.removeFromCollection(collectionId, itemId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['collection'] });
      qc.invalidateQueries({ queryKey: ['collections'] });
    },
  });
  return (
    <Reorder.Group axis="y" values={items} onReorder={setItems} className="max-w-2xl space-y-1.5">
      {items.map((it) => (
        <ArrangeRow key={it.id} item={it} onDrop={() => save.mutate(items.map((i) => i.id))} onRemove={() => removeItem.mutate(it.id)} />
      ))}
    </Reorder.Group>
  );
}

function ArrangeRow({ item, onDrop, onRemove }: { item: Item; onDrop: () => void; onRemove: () => void }) {
  const drag = useDragControls();
  return (
    <Reorder.Item
      value={item}
      dragListener={false}
      dragControls={drag}
      onDragEnd={onDrop}
      className="flex items-center gap-3 rounded-2xl bg-surface p-2 pr-3 ring-1 ring-border"
      whileDrag={{ scale: 1.02, boxShadow: '0 12px 30px rgba(0,0,0,0.35)' }}
    >
      <button type="button" aria-label={`Drag ${item.title}`} onPointerDown={(e) => drag.start(e)} className="cursor-grab touch-none p-1.5 text-subtle hover:text-foreground">
        <GripVertical className="size-4" />
      </button>
      <Artwork src={item.images?.primary} alt="" className="w-9 shrink-0 rounded-md" fallbackTitle={item.title} />
      <p className="min-w-0 flex-1 truncate text-sm font-medium">
        {item.title} {item.year ? <span className="font-normal text-muted">({item.year})</span> : null}
      </p>
      <Button variant="ghost" size="icon" aria-label={`Take ${item.title} out`} onClick={onRemove}>
        <X />
      </Button>
    </Reorder.Item>
  );
}
