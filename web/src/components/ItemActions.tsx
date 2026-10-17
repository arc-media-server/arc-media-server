import { useState } from 'react';
import { Layers, MoreHorizontal, Pencil, SearchCheck } from 'lucide-react';
import type { Item } from '@/lib/api';
import { EditDetailsDialog } from '@/components/EditDetails';
import { FixMatchDialog } from '@/components/FixMatch';
import { AddToCollectionDialog } from '@/components/Collections';
import { useLibraries } from '@/components/Sidebar';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

type Dialog = 'edit' | 'match' | 'collection' | null;

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
    </>
  );
  return { open: setOpen, dialogs };
}

/** The "⋯" button on a movie or show page. */
export function MoreMenu({ onOpen }: { onOpen: (d: Exclude<Dialog, null>) => void }) {
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
        <DropdownMenuItem onSelect={() => onOpen('match')}>
          <SearchCheck className="size-4 text-muted" /> Fix Match…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
