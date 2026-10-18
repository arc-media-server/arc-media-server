import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Gauge, Layers, MoreHorizontal, Pencil, SearchCheck, Split } from 'lucide-react';
import { OptimizeDialog } from '@/components/Optimize';
import { api, type Item } from '@/lib/api';
import { isLocalVideo } from '@/lib/kinds';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { EditDetailsDialog } from '@/components/EditDetails';
import { FixMatchDialog } from '@/components/FixMatch';
import { AddToCollectionDialog } from '@/components/Collections';
import { useLibraries } from '@/components/Sidebar';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

type Dialog = 'edit' | 'match' | 'collection' | 'split' | 'optimize' | null;

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
      <EditDetailsDialog item={item} open={open === 'edit'} onOpenChange={(o) => setOpen(o ? 'edit' : null)} />
      <FixMatchDialog item={item} open={open === 'match'} onOpenChange={(o) => setOpen(o ? 'match' : null)} />
      <AddToCollectionDialog item={item} library={library} open={open === 'collection'} onOpenChange={(o) => setOpen(o ? 'collection' : null)} />
      <SplitDialog item={item} open={open === 'split'} onOpenChange={(o) => setOpen(o ? 'split' : null)} />
      <OptimizeDialog items={[item]} open={open === 'optimize'} onOpenChange={(o) => setOpen(o ? 'optimize' : null)} />
    </>
  );
  return { open: setOpen, dialogs };
}

/** The "⋯" button on a movie or show page. */
export function MoreMenu({ item, onOpen }: { item?: Item; onOpen: (d: Exclude<Dialog, null>) => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="secondary" size="lg" className="w-12 px-0" aria-label="More">
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
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
        {item && !isLocalVideo(item) && (
          <DropdownMenuItem onSelect={() => onOpen('optimize')}>
            <Gauge className="size-4 text-muted" /> Optimize…
          </DropdownMenuItem>
        )}
        {item && splitVersions(item) > 1 && (
          <DropdownMenuItem onSelect={() => onOpen('split')}>
            <Split className="size-4 text-muted" /> Split Apart…
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
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
