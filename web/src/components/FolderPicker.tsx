import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ChevronLeft, ChevronRight, Folder, HardDrive, House } from 'lucide-react';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

/**
 * Browses the server's folders to choose one. Starts at `start` (or the list of drives) and calls
 * onChoose with the folder being looked at.
 */
export function FolderPicker({ start, onChoose, onCancel }: { start?: string; onChoose: (path: string) => void; onCancel: () => void }) {
  const [path, setPath] = useState(start ?? '');
  const [typed, setTyped] = useState(start ?? '');
  const listing = useQuery({ queryKey: ['folders', path], queryFn: () => api.folders(path || undefined) });

  const go = (p: string) => {
    setPath(p);
    setTyped(p);
  };

  return (
    <div className="overflow-hidden rounded-2xl bg-surface-2/50 ring-1 ring-border">
      <form
        className="flex items-center gap-2 border-b border-border p-2"
        onSubmit={(e) => {
          e.preventDefault();
          go(typed.trim());
        }}
      >
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-8 shrink-0 rounded-lg"
          aria-label="Up one folder"
          disabled={listing.data?.parent === null || !path}
          onClick={() => go(listing.data?.parent ?? '')}
        >
          <ChevronLeft />
        </Button>
        <input
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          placeholder="Choose a drive, or type a folder path"
          className="h-8 min-w-0 flex-1 rounded-lg bg-transparent px-2 font-mono text-xs outline-none placeholder:font-sans placeholder:text-subtle focus:bg-surface-3"
        />
      </form>
      <div className="h-56 overflow-y-auto p-1">
        {listing.error && <p className="p-3 text-sm text-warning">{listing.error.message}</p>}
        {listing.data?.folders.length === 0 && <p className="p-3 text-sm text-muted">No folders inside this one.</p>}
        {listing.data?.folders.map((f) => {
          const Icon = !path ? (f.name === 'Home' ? House : HardDrive) : Folder;
          return (
            <button
              key={f.path}
              type="button"
              onClick={() => go(f.path)}
              className="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-left text-sm transition-colors hover:bg-surface-3"
            >
              <Icon className={cn('size-4 shrink-0', path ? 'text-accent' : 'text-muted')} />
              <span className="min-w-0 flex-1 truncate">{f.name}</span>
              <ChevronRight className="size-3.5 shrink-0 text-subtle" />
            </button>
          );
        })}
      </div>
      <div className="flex items-center justify-end gap-2 border-t border-border p-2">
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="button" variant="primary" size="sm" disabled={!path || !!listing.error} onClick={() => onChoose(path)}>
          Use This Folder
        </Button>
      </div>
    </div>
  );
}
