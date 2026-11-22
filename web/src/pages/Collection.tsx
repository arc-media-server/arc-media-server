import { useTranslation } from 'react-i18next';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Reorder, useDragControls } from 'motion/react';
import { ArrowUpDown, Check, GripVertical, Loader2, MoreHorizontal, Pencil, Play, Shuffle, Trash2, X } from 'lucide-react';
import { usePlayer } from '@/components/Player';
import { collectionQueue, shuffled } from '@/lib/shuffle';
import { api, type Item } from '@/lib/api';
import { kindUnit } from '@/lib/kinds';
import { useIsAdmin } from '@/lib/auth';
import { countUnit } from '@/lib/browsing-labels';
import { Artwork } from '@/components/Artwork';
import { CollectionDialog } from '@/components/Collections';
import { Hero } from '@/components/Hero';
import { ErrorState, Page } from '@/components/Page';
import { PosterGrid } from '@/components/PosterCard';
import { useLibraries } from '@/components/Sidebar';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

export function CollectionPage() {
  const { t } = useTranslation('collection');
  const kindLabels = { auto: t('collection:kind.collection'), manual: t('collection:kind.collection'), smart: t('collection:kind.smart'), folder: t('collection:kind.folder') };
  const { id } = useParams();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const admin = useIsAdmin();
  const { data, error } = useQuery({ queryKey: ['collection', id], queryFn: () => api.collection(id!) });
  const { data: libraries } = useLibraries();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [arranging, setArranging] = useState(false);
  const play = usePlayer();
  const [starting, setStarting] = useState(false);
  // Play the collection in its order, or shuffled (its shows' episodes too).
  const start = async (shuffle: boolean) => {
    if (!data) return;
    setStarting(true);
    try {
      const q = await collectionQueue(data.items);
      const list = shuffle ? shuffled(q) : q;
      if (list.length) play(list[0], list);
    } finally {
      setStarting(false);
    }
  };

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
      <Hero backdrop={c.images.backdrop_full ?? c.images.backdrop}className="min-h-[52vh]">
        <div className="flex min-h-[52vh] items-end gap-8 px-6 pb-10 pt-24 md:px-10">
          <Artwork src={c.images.primary} alt={c.title} fallbackTitle={c.title} eager className="hidden w-44 shrink-0 rounded-2xl shadow-2xl shadow-black/50 ring-1 ring-white/10 lg:block" />
          <div className="max-w-2xl">
            <p className="mb-2 text-xs font-semibold uppercase tracking-[0.18em] text-accent">{kindLabels[c.kind]}</p>
            <h1 className="text-4xl font-bold leading-[1.05] md:text-5xl">{c.title}</h1>
            <p className="mt-3 text-sm text-foreground/70">{countUnit(c.count, unit)}</p>
            {c.overview && <p className="mt-4 text-[15px] leading-relaxed text-foreground/85">{c.overview}</p>}
            {(data.items.length > 0 || admin) && (
              <div className="mt-6 flex flex-wrap items-center gap-3">
                {data.items.length > 0 && (
                  <>
                    <Button variant="primary" size="lg" disabled={starting} onClick={() => void start(false)}>
                      <Play className="fill-current" /> {t('common:actions.play')}
                    </Button>
                    <Button variant="secondary" size="lg" className="size-12 rounded-2xl px-0" disabled={starting} onClick={() => void start(true)} title={t('common:actions.shuffle')} aria-label={t('common:actions.shuffle')}>
                      {starting ? <Loader2 className="animate-spin" /> : <Shuffle />}
                    </Button>
                  </>
                )}
                {admin && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="secondary" size="lg" className="size-12 rounded-2xl px-0" title={t('titles:itemActions.more')} aria-label={t('titles:itemActions.more')}>
                        <MoreHorizontal />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start" className="min-w-48">
                      <DropdownMenuItem onSelect={() => setEditing(true)}><Pencil className="size-4 text-muted" /> {t('common:actions.edit')}</DropdownMenuItem>
                      {c.kind === 'manual' && data.items.length > 1 && !arranging && (
                        <DropdownMenuItem onSelect={() => setArranging(true)}><ArrowUpDown className="size-4 text-muted" /> {t('collection:arrange')}</DropdownMenuItem>
                      )}
                      <DropdownMenuSeparator />
                      <DropdownMenuItem onSelect={() => setDeleting(true)}><Trash2 className="size-4 text-muted" /> {c.kind === 'auto' || c.kind === 'folder' ? t('common:actions.remove') : t('common:actions.delete')}</DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
                {arranging && (
                  <Button variant="secondary" size="lg" onClick={() => setArranging(false)}>
                    <Check /> {t('common:actions.done')}
                  </Button>
                )}
              </div>
            )}
          </div>
        </div>
      </Hero>

      <div className="px-6 pt-8 md:px-10">
        {arranging ? <Arrange collectionId={c.id} items={data.items} /> : data.items.length > 0 && <PosterGrid items={data.items} landscape={c.kind === 'folder'} />}
        {!arranging && data.items.length === 0 && (
          <p className="text-sm text-muted">{c.kind === 'manual' ? unit === 'show' ? t('collection:empty.show') : unit === 'video' ? t('collection:empty.video') : t('collection:empty.movie') : t('collection:empty.matches')}</p>
        )}
      </div>

      {library && <CollectionDialog library={library} collection={c} open={editing} onOpenChange={setEditing} />}
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={c.kind === 'auto' ? t('collection:removeTitle', { title: c.title }) : t('collection:deleteTitle', { title: c.title })}
        confirmLabel={c.kind === 'auto' ? t('common:actions.remove') : t('common:actions.delete')}
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => remove.mutate()}
      >
        <p>
          {c.kind === 'auto'
            ? t('collection:removeDescription')
            : t('collection:deleteDescription')}
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
  const { t } = useTranslation('collection');
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
      <button type="button" aria-label={t('collection:dragTitle', { title: item.title })} onPointerDown={(e) => drag.start(e)} className="cursor-grab touch-none p-1.5 text-subtle hover:text-foreground">
        <GripVertical className="size-4" />
      </button>
      <Artwork src={item.images?.primary} alt="" className="w-9 shrink-0 rounded-md" fallbackTitle={item.title} />
      <p className="min-w-0 flex-1 truncate text-sm font-medium">
        {item.title} {item.year ? <span className="font-normal text-muted">({item.year})</span> : null}
      </p>
      <Button variant="ghost" size="icon" aria-label={t('collection:takeOut', { title: item.title })} onClick={onRemove}>
        <X />
      </Button>
    </Reorder.Item>
  );
}
