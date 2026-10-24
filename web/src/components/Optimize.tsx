import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, Check, Loader2, Trash2 } from 'lucide-react';
import { api, type Item } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { ErrorNotice } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';

const choices = [
  [1080, '1080p', 'Full HD, about 3.5 GB an hour'],
  [720, '720p', 'For phones and slow connections, about 1.8 GB an hour'],
] as const;

/** Optimize: make easy-to-play copies of titles ahead of time, as Plex does. */
export function OptimizeDialog({ items, open, onOpenChange }: { items: Item[]; open: boolean; onOpenChange: (o: boolean) => void }) {
  return <Dialog open={open} onOpenChange={onOpenChange}>{open && <OptimizeForm items={items} onDone={() => onOpenChange(false)} />}</Dialog>;
}

function OptimizeForm({ items, onDone }: { items: Item[]; onDone: () => void }) {
  const qc = useQueryClient();
  const [quality, setQuality] = useState<1080 | 720>(1080);
  const run = useMutation({
    mutationFn: () => api.optimize(items.map((i) => i.id), quality),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['optimized'] });
      qc.invalidateQueries({ queryKey: ['activity'] });
    },
  });
  const what = items.length === 1 ? items[0].title : fmt.plural(items.length, 'title');
  return (
    <DialogContent
      title="Optimize"
      description={`Make an easy-to-play copy of ${what} ahead of time, so players that can't play ${items.length === 1 ? 'it' : 'them'} as ${items.length === 1 ? 'it is' : 'they are'} (or remote ones) don't have to wait for it to be converted. Copies are kept in Arc's data folder; your files aren't touched.`}
      footer={
        run.isSuccess ? (
          <Button variant="primary" onClick={onDone}>
            Done
          </Button>
        ) : (
          <>
            {run.error && <p className="mr-auto text-sm text-warning">{run.error.message}</p>}
            <DialogClose asChild>
              <Button variant="ghost">Cancel</Button>
            </DialogClose>
            <Button variant="primary" disabled={run.isPending} onClick={() => run.mutate()}>
              {run.isPending && <Loader2 className="animate-spin" />} Optimize
            </Button>
          </>
        )
      }
    >
      {run.isSuccess ? (
        <p className="pb-2 text-sm text-muted">
          {run.data.queued
            ? `${fmt.plural(run.data.queued, 'copy', 'copies')} queued. They're made one at a time in the background; follow them in Activity.`
            : 'Those copies are already made or on their way.'}
        </p>
      ) : (
        <div className="grid gap-2 pb-2 sm:grid-cols-2">
          {choices.map(([q, label, hint]) => (
            <button
              key={q}
              type="button"
              aria-pressed={quality === q}
              onClick={() => setQuality(q)}
              className={cn(
                'rounded-xl px-3 py-2.5 text-left ring-1 transition-colors',
                quality === q ? 'bg-accent/10 ring-2 ring-accent' : 'bg-surface-2/50 ring-border hover:bg-surface-2',
              )}
            >
              <span className="flex items-center gap-1.5 text-sm font-medium">
                {quality === q && <Check className="size-3.5 text-accent" />}
                {label}
              </span>
              <span className="block text-xs text-subtle">{hint}</span>
            </button>
          ))}
        </div>
      )}
    </DialogContent>
  );
}

/** Settings: the optimized copies, what they take up, and removing them. */
export function OptimizedList() {
  const qc = useQueryClient();
  const optimized = useQuery({
    queryKey: ['optimized'],
    queryFn: api.optimized,
    refetchInterval: (q) => (q.state.data?.versions.some((v) => v.status === 'queued' || v.status === 'converting') ? 3000 : false),
  });
  const { data } = optimized;
  const feedback = optimized.error && <div className="m-4 space-y-2">
    {data && <p className="text-xs text-muted">Showing the last loaded optimized copies. They may be out of date.</p>}
    <ErrorNotice error={optimized.error} onRetry={() => void optimized.refetch()} busy={optimized.isFetching} />
  </div>;
  const remove = useMutation({
    mutationFn: (v: { file_id: number; quality: number }) => api.deleteOptimized(v.file_id, v.quality),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['optimized'] }),
  });
  if (!data) return feedback || <div role="status" className="m-4 h-12 animate-pulse rounded-xl bg-surface-2"><span className="sr-only">Loading optimized copies…</span></div>;
  if (data.versions.length === 0) {
    return (
      <>{feedback}<p className="px-4 py-3.5 text-xs leading-relaxed text-muted">
        None yet. Choose Optimize from a title&apos;s ⋯ menu, or Select titles in a library, to make easy-to-play copies ahead of time.
      </p></>
    );
  }
  return (
    <>
      {feedback}
      <p className="px-4 py-3 text-xs text-muted">
        {fmt.plural(data.versions.length, 'copy', 'copies')} · {fmt.fileSize(data.total_size)} in Arc&apos;s data folder
      </p>
      {data.versions.map((v) => (
        <div key={`${v.file_id}-${v.quality}`} className="flex items-center gap-3 px-4 py-2.5">
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">
              {v.series_title ? `${v.series_title}: ${v.title}` : v.title} <span className="font-normal text-muted">· {v.quality}p</span>
            </p>
            <p className={cn('flex items-center gap-1 truncate text-xs', v.status === 'failed' ? 'text-warning' : 'text-subtle')}>
              {v.status === 'failed' && <AlertCircle className="size-3.5 shrink-0" />}
              {v.status === 'done'
                ? fmt.fileSize(v.size)
                : v.status === 'converting'
                  ? 'Being made now'
                  : v.status === 'queued'
                    ? 'Waiting'
                    : (v.error ?? 'Failed')}
            </p>
          </div>
          <Button variant="ghost" size="sm" aria-label="Delete" disabled={remove.isPending} onClick={() => remove.mutate(v)}>
            <Trash2 />
          </Button>
        </div>
      ))}
    </>
  );
}
