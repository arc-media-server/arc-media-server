import { useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Loader2, RotateCcw, Upload } from 'lucide-react';
import { api, type ArtworkType, type Item } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';

/** Choose a poster or background from TMDB's, upload one, or go back to automatic. */
export function ChooseArtworkDialog({ item, open, onOpenChange }: { item: Item; open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <Chooser item={item} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function Chooser({ item, onDone }: { item: Item; onDone: () => void }) {
  const qc = useQueryClient();
  const [type, setType] = useState<ArtworkType>('poster');
  const file = useRef<HTMLInputElement>(null);
  const choices = useQuery({ queryKey: ['artwork', item.id], queryFn: () => api.artwork(item.id) });

  const done = () => {
    for (const key of ['item', 'items', 'seasons', 'artwork', 'featured', 'continue', 'next-up']) qc.invalidateQueries({ queryKey: [key] });
    onDone();
  };
  const choose = useMutation({ mutationFn: (path: string | null) => api.chooseArtwork(item.id, type, path), onSuccess: done });
  const upload = useMutation({ mutationFn: (f: File) => api.uploadArtwork(item.id, type, f), onSuccess: done });
  const busy = choose.isPending || upload.isPending;
  const error = choose.error ?? upload.error;

  const c = choices.data;
  const images = type === 'poster' ? c?.posters : c?.backdrops;
  const current = c?.current[type];
  const chosen = c?.chosen[type];

  return (
    <DialogContent
      title="Choose Artwork"
      description={
        c?.local[type] && !chosen
          ? `Arc is using the ${type === 'poster' ? 'poster' : 'background'} image in this title's folder. Choosing one here replaces it in Arc (the file isn't changed).`
          : 'Pick an image from TMDB or upload your own. Your choice is kept when the details are refreshed.'
      }
      className="w-[min(860px,calc(100vw-32px))]"
      footer={
        <>
          {error && <p className="mr-auto text-sm text-warning">{error.message}</p>}
          {chosen && (
            <Button variant="ghost" disabled={busy} onClick={() => choose.mutate(null)}>
              <RotateCcw /> Automatic
            </Button>
          )}
          <input
            ref={file}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            hidden
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) upload.mutate(f);
              e.target.value = '';
            }}
          />
          <Button variant="secondary" disabled={busy} onClick={() => file.current?.click()}>
            {upload.isPending ? <Loader2 className="animate-spin" /> : <Upload />} Upload…
          </Button>
        </>
      }
    >
      <div className="mb-4 flex gap-1">
        {(['poster', 'backdrop'] as const).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => setType(t)}
            className={cn('rounded-full px-3.5 py-1 text-[13px] font-medium transition-colors', type === t ? 'bg-foreground text-background' : 'text-muted hover:text-foreground')}
          >
            {t === 'poster' ? 'Poster' : 'Background'}
          </button>
        ))}
      </div>
      {choices.error && <p className="text-sm text-warning">{choices.error.message}</p>}
      {!c && !choices.error && <div className="h-64 animate-pulse rounded-2xl bg-surface-2" />}
      {images?.length === 0 && <p className="py-10 text-center text-sm text-muted">TMDB has no other images for this title. You can upload one.</p>}
      <div className={cn('grid gap-3 pb-2', type === 'poster' ? 'grid-cols-[repeat(auto-fill,minmax(120px,1fr))]' : 'grid-cols-[repeat(auto-fill,minmax(220px,1fr))]')}>
        {images?.map((img) => {
          const selected = img.path === current;
          return (
            <button
              key={img.path}
              type="button"
              disabled={busy}
              onClick={() => choose.mutate(img.path)}
              className={cn(
                'group relative overflow-hidden rounded-xl bg-surface-2 ring-1 ring-border transition-all hover:-translate-y-0.5 hover:ring-2 hover:ring-accent/60',
                type === 'poster' ? 'aspect-[2/3]' : 'aspect-video',
                selected && 'ring-2 ring-accent',
              )}
              title={`${img.width}×${img.height}${img.language ? ` · ${img.language}` : ''}`}
            >
              <img src={img.thumb} alt="" loading="lazy" className="size-full object-cover" />
              {selected && (
                <span className="absolute right-1.5 top-1.5 flex size-6 items-center justify-center rounded-full bg-accent-strong text-white shadow">
                  <Check className="size-3.5" strokeWidth={3} />
                </span>
              )}
              {choose.isPending && choose.variables === img.path && (
                <span className="absolute inset-0 flex items-center justify-center bg-black/40">
                  <Loader2 className="size-6 animate-spin text-white" />
                </span>
              )}
            </button>
          );
        })}
      </div>
    </DialogContent>
  );
}
