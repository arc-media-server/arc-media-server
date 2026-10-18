import { useState } from 'react';
import { Link } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Popover } from 'radix-ui';
import { Bell } from 'lucide-react';
import { api, itemHref, posterUrl } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Artwork } from '@/components/Artwork';

/** "New for you": new episodes of shows the profile watches, and new movies, with a count of unread. */
export function NewsButton({ className }: { className?: string }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const { data } = useQuery({ queryKey: ['news'], queryFn: api.news, refetchInterval: 5 * 60_000 });
  const seen = useMutation({ mutationFn: api.newsSeen, onSuccess: () => qc.invalidateQueries({ queryKey: ['news'] }) });
  const unread = data?.unread ?? 0;
  return (
    <Popover.Root
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        // Closing it marks what was shown as seen.
        if (!o && unread > 0) seen.mutate();
      }}
    >
      <Popover.Trigger
        aria-label={unread ? `New for you: ${unread} new` : 'New for you'}
        className={cn(
          'relative flex size-8 items-center justify-center rounded-full text-muted outline-none transition-colors hover:bg-surface-3 hover:text-foreground focus-visible:ring-2 focus-visible:ring-accent',
          className,
        )}
      >
        <Bell className="size-[18px]" />
        {unread > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[10px] font-semibold text-white">
            {unread > 9 ? '9+' : unread}
          </span>
        )}
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={8}
          collisionPadding={12}
          className="animate-menu-in z-50 w-[360px] max-w-[calc(100vw-24px)] overflow-hidden rounded-2xl border border-border bg-surface shadow-2xl shadow-black/40"
        >
          <div className="border-b border-border px-4 py-3">
            <p className="text-sm font-semibold">New for You</p>
            <p className="text-xs text-subtle">New episodes of shows you watch, and new movies, from the last month.</p>
          </div>
          <div className="max-h-[min(65vh,480px)] overflow-y-auto p-1.5">
            {!data ? (
              <div className="m-2 h-16 animate-pulse rounded-xl bg-surface-2" />
            ) : data.entries.length === 0 ? (
              <p className="px-3 py-6 text-center text-sm text-muted">Nothing new yet. New episodes of shows you watch will show up here.</p>
            ) : (
              data.entries.map((e) => (
                <Link
                  key={e.item.id}
                  to={itemHref(e.item)}
                  onClick={() => setOpen(false)}
                  className="flex items-center gap-3 rounded-xl px-2.5 py-2 transition-colors hover:bg-surface-2"
                >
                  <div className="w-9 shrink-0 overflow-hidden rounded-md">
                    <Artwork src={posterUrl(e.item)} alt="" fallbackTitle={e.item.title} icon={e.item.kind === 'series' ? 'tv' : 'film'} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{e.item.title}</p>
                    <p className="truncate text-xs text-subtle">
                      {e.item.kind === 'series' ? (e.count === 1 ? 'A new episode' : `${e.count} new episodes`) : e.item.metadata_state === 'locked' ? 'New video' : 'New movie'} · {ago(e.added_at)}
                    </p>
                  </div>
                  {e.unread && <span className="size-2 shrink-0 rounded-full bg-accent" aria-label="New" />}
                </Link>
              ))
            )}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

/** "2 h ago", "yesterday", "5 days ago". */
function ago(unix: number) {
  const s = Date.now() / 1000 - unix;
  if (s < 3600) return `${Math.max(1, Math.floor(s / 60))} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`;
  const d = Math.floor(s / 86400);
  return d === 1 ? 'yesterday' : `${d} days ago`;
}
