import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { Link } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Popover } from 'radix-ui';
import { Bell } from 'lucide-react';
import { api, itemHref, posterUrl } from '@/lib/api';
import { cn } from '@/lib/utils';
import { ErrorNotice } from '@/components/Page';
import { Artwork } from '@/components/Artwork';

/** "New for you": new episodes of shows the profile watches, and new movies, with a count of unread. */
export function NewsButton({ className }: { className?: string }) {
  const { t } = useTranslation('common');
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const news = useQuery({ queryKey: ['news'], queryFn: api.news, refetchInterval: 5 * 60_000 });
  const { data } = news;
  const seen = useMutation({ mutationFn: api.newsSeen, onSuccess: () => qc.invalidateQueries({ queryKey: ['news'] }) });
  const unread = data?.unread ?? 0;
  return (
    <Popover.Root
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        // Closing it marks what was shown as seen.
        if (!o && unread > 0 && !news.error) seen.mutate();
      }}
    >
      <Popover.Trigger
        aria-label={unread ? t('news.unread', { count: unread }) : t('news.label')}
        className={cn(
          'relative flex size-8 items-center justify-center rounded-full text-muted outline-none transition-colors hover:bg-surface-3 hover:text-foreground focus-visible:ring-2 focus-visible:ring-accent',
          className,
        )}
      >
        <Bell className="size-[18px]" />
        {unread > 0 && (
          <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-accent-strong px-1 text-[10px] font-semibold text-white">
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
            <p className="text-sm font-semibold">{t('news.heading')}</p>
            <p className="text-xs text-subtle">{t('news.description')}</p>
          </div>
          <div className="max-h-[min(65vh,480px)] overflow-y-auto p-1.5">
            {news.error && <div className="m-2 space-y-2">
              {data && <p className="text-xs text-muted">{t('news.stale')}</p>}
              <ErrorNotice error={news.error} onRetry={() => void news.refetch()} busy={news.isFetching} />
            </div>}
            {!data ? (news.error ? null :
              <div role="status" className="m-2 h-16 animate-pulse rounded-xl bg-surface-2"><span className="sr-only">{t('news.loading')}</span></div>
            ) : data.entries.length === 0 ? (
              <p className="px-3 py-6 text-center text-sm text-muted">{t('news.empty')}</p>
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
                      {e.item.kind === 'series' ? (t('news.episodes', { count: e.count })) : e.item.metadata_state === 'locked' ? t('news.video') : t('news.movie')} · {ago(e.added_at)}
                    </p>
                  </div>
                  {e.unread && <span className="size-2 shrink-0 rounded-full bg-accent" aria-label={t('news.new')} />}
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
  if (s < 3600) return i18n.t('common:time.minutesAgo', { count: Math.max(1, Math.floor(s / 60)) });
  if (s < 86400) return i18n.t('common:time.hoursAgo', { count: Math.floor(s / 3600) });
  const d = Math.floor(s / 86400);
  return i18n.t('common:time.newsDaysAgo', { count: d });
}
