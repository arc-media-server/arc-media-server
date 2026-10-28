import i18n from '@/i18n';
import { Trans, useTranslation } from 'react-i18next';
import { useState, type ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router';
import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { motion } from 'motion/react';
import { Check, ChartColumn, HardDrive, History, Loader2 } from 'lucide-react';
import { api, itemHref, type Bucket, type Item, type Play, type RankedTitle, type StatsContext, type TitleSize } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { Artwork } from '@/components/Artwork';
import { Avatar } from '@/components/Avatar';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { inputClass } from '@/components/ui/field';

const tabs = [
  { id: 'activity', get label() { return i18n.t('reports:navigation.activity'); } },
  { id: 'history', get label() { return i18n.t('reports:navigation.history'); } },
  { id: 'storage', get label() { return i18n.t('reports:navigation.storage'); } },
] as const;
type Tab = (typeof tabs)[number]['id'];

const periods = [
  { days: 7, get label() { return i18n.t('reports:navigation.lastWeek'); } },
  { days: 30, get label() { return i18n.t('reports:navigation.lastMonth'); } },
  { days: 90, get label() { return i18n.t('reports:navigation.lastQuarter'); } },
  { days: 365, get label() { return i18n.t('reports:navigation.lastYear'); } },
  { days: 0, get label() { return i18n.t('reports:navigation.allTime'); } },
];

/** Admin insights: what's been watched and by whom, watch history, and what takes up space. */
export function InsightsPage() {
  const { t } = useTranslation('reports');
  const [params, setParams] = useSearchParams();
  const tab = (tabs.find((tabId) => tabId.id === params.get('tab'))?.id ?? 'activity') as Tab;
  const set = (key: string, value: string) =>
    setParams(
      (p) => {
        if (value) p.set(key, value);
        else p.delete(key);
        return p;
      },
      { replace: true },
    );
  const profile = params.get('profile') ?? '';
  const library = Number(params.get('library')) || undefined;
  const days = params.has('days') ? Number(params.get('days')) : 30;

  return (
    <Page className="pb-16">
      <PageHeader title={t("reports:insights.insights")} subtitle={t("reports:insights.whatSBeingWatched")} />
      <div className="sticky top-0 z-30 border-b border-border bg-background/80 backdrop-blur-2xl">
        <div className="flex flex-wrap items-center gap-3 px-6 py-3 md:px-10">
          <div className="flex gap-1">
            {tabs.map((tabId) => (
              <button
                key={tabId.id}
                onClick={() => set('tab', tabId.id === 'activity' ? '' : tabId.id)}
                className={cn('relative rounded-full px-4 py-1.5 text-[13.5px] font-medium transition-colors', tab === tabId.id ? 'text-background' : 'text-muted hover:text-foreground')}
              >
                {tab === tabId.id && <motion.span layoutId="insights-tab" className="absolute inset-0 rounded-full bg-foreground" transition={{ type: 'spring', bounce: 0.15, duration: 0.45 }} />}
                <span className="relative">{tabId.label}</span>
              </button>
            ))}
          </div>
          <div className="ml-auto flex flex-wrap gap-2">
            <Filters tab={tab} profile={profile} library={library} days={days} set={set} />
          </div>
        </div>
      </div>
      <div className="px-6 pt-8 md:px-10">
        {tab === 'activity' && <Activity profile={profile} library={library} days={days} />}
        {tab === 'history' && <HistoryList profile={profile} library={library} />}
        {tab === 'storage' && <Storage library={library} />}
      </div>
    </Page>
  );
}

function Filters({ tab, profile, library, days, set }: { tab: Tab; profile: string; library?: number; days: number; set: (key: string, value: string) => void }) {
  const { t } = useTranslation('reports');
  // The lists of profiles and libraries come with any stats response; the cheapest is one history row.
  const context = useQuery({ queryKey: ['stats-context'], queryFn: () => api.statsHistory({ before: 1 }), staleTime: 60_000 });
  const select = cn(inputClass, 'h-9 w-auto pr-8');
  return (
    <>
      {tab !== 'storage' && (
        <select className={select} value={profile} onChange={(e) => set('profile', e.target.value)} aria-label={t("reports:insights.profile")}>
          <option value="">{t("reports:insights.allProfiles")}</option>
          {context.data?.profiles.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      )}
      <select className={select} value={library ?? ''} onChange={(e) => set('library', e.target.value)} aria-label={t("reports:insights.library")}>
        <option value="">{t("reports:insights.allLibraries")}</option>
        {context.data?.libraries.map((l) => (
          <option key={l.id} value={l.id}>
            {l.name}
          </option>
        ))}
      </select>
      {tab === 'activity' && (
        <select className={select} value={days} onChange={(e) => set('days', e.target.value === '30' ? '' : e.target.value)} aria-label={t("reports:insights.period")}>
          {periods.map((p) => (
            <option key={p.days} value={p.days}>
              {p.label}
            </option>
          ))}
        </select>
      )}
    </>
  );
}

// Activity

function Activity({ profile, library, days }: { profile: string; library?: number; days: number }) {
  const { t } = useTranslation('reports');
  const stats = useQuery({
    queryKey: ['stats', profile, library, days],
    queryFn: () => api.stats({ profile, library, days }),
    placeholderData: keepPreviousData,
  });
  if (stats.error) return <ErrorState error={stats.error} />;
  const d = stats.data;
  if (!d) return <div className="h-96 animate-pulse rounded-2xl bg-surface" />;
  if (d.total.plays === 0) {
    return (
      <EmptyState icon={<ChartColumn />} title={t("reports:insights.nothingWatchedYet")}>
        {days ? t("reports:insights.nothingWasWatchedIn") : t("reports:insights.oncePeopleStartWatching")} {t("reports:insights.arcCountsAViewing")}</EmptyState>
    );
  }
  const profileOf = (id: string) => d.profiles.find((p) => p.id === id);
  const libraryOf = (id: string) => d.libraries.find((l) => String(l.id) === id);

  return (
    <div className={cn('space-y-10 transition-opacity', stats.isPlaceholderData && 'opacity-60')}>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Tile label={t("reports:insights.watchTime")} value={fmt.watchTime(d.total.watched_ms)} />
        <Tile label={t("reports:insights.plays")} value={fmt.number(d.total.plays)} />
        <Tile label={t("reports:insights.titles")} value={fmt.number(d.titles)} />
        <Tile label={t("reports:insights.profilesWatching")} value={fmt.number(d.profiles_watching, { useGrouping: false })} />
      </div>

      <Section title={t("reports:insights.watchTime")} note={d.step === 'day' ? t('reports:insights.byDay') : d.step === 'week' ? t('reports:insights.byWeek') : t('reports:insights.byMonth')}>
        <Timeline points={d.timeline} step={d.step} />
      </Section>

      <div className="grid gap-10 lg:grid-cols-2">
        <Section title={t("reports:insights.mostWatchedMovies")}>
          <TopList titles={d.movies} items={d.items} empty={t('reports:insights.noMovies')} />
        </Section>
        <Section title={t("reports:insights.mostWatchedShows")}>
          <TopList titles={d.shows} items={d.items} empty={t('reports:insights.noShows')} />
        </Section>
      </div>

      <div className="grid gap-10 lg:grid-cols-2">
        <Section title={t("reports:insights.busiestTimes")} note={t('reports:insights.viewingStarts')}>
          <Heatmap hours={d.hours} />
        </Section>
        <Section title={t("reports:insights.genres")}>
          <Bars rows={d.genres.map((g) => ({ key: g.name, label: g.name, bucket: g }))} />
        </Section>
      </div>

      <div className="grid gap-10 lg:grid-cols-3">
        {!profile && (
          <Section title={t("reports:insights.profiles")}>
            <Bars
              rows={d.by_profile.map((b) => {
                const p = profileOf(b.id);
                return {
                  key: b.id,
                  label: (
                    <span className="flex items-center gap-2">
                      <Avatar name={p?.name ?? '?'} color={p?.color ?? 'violet'} className="size-5 text-[10px]" />
                      {p?.name ?? t("reports:insights.removedProfile")}
                    </span>
                  ),
                  bucket: b,
                };
              })}
            />
          </Section>
        )}
        {!library && (
          <Section title={t("reports:insights.libraries")}>
            <Bars rows={d.by_library.map((b) => ({ key: b.id, label: libraryOf(b.id)?.name ?? t('reports:insights.removedLibrary'), bucket: b }))} />
          </Section>
        )}
        <Section title={t("reports:insights.devices")}>
          <Bars rows={d.devices.map((b) => ({ key: b.name, label: b.name, bucket: b }))} />
        </Section>
        <Section title={t("reports:insights.howItPlayed")} note={t('reports:insights.conversionPower')}>
          <Bars rows={d.methods.map((b) => ({ key: b.name, label: methodName(b.name), bucket: b }))} />
        </Section>
      </div>
    </div>
  );
}

function methodName(m?: string) {
  return m === 'transcode' ? i18n.t('reports:methods.converted') : m === 'remux' ? i18n.t('reports:methods.remux') : i18n.t('reports:methods.direct');
}

function Tile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl bg-surface p-5 ring-1 ring-border">
      <p className="text-[13px] text-muted">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums tracking-tight md:text-3xl">{value}</p>
    </div>
  );
}

function Section({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section>
      <div className="mb-4 flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
        <h2 className="shrink-0 text-lg font-semibold">{title}</h2>
        {note && <span className="text-[13px] text-subtle">{note}</span>}
      </div>
      {children}
    </section>
  );
}

function Timeline({ points, step }: { points: (Bucket & { date: string })[]; step: string }) {
  const { t } = useTranslation('reports');
  const most = Math.max(1, ...points.map((p) => p.watched_ms));
  const label = (iso: string) =>
    new Date(`${iso}T00:00:00`).toLocaleDateString(fmt.formatLocale(), step === 'month' ? { month: 'short', year: 'numeric' } : { day: 'numeric', month: 'short' });
  const ticks = points.length > 1 ? [0, Math.floor((points.length - 1) / 2), points.length - 1] : [0];
  return (
    <div className="rounded-2xl bg-surface p-5 ring-1 ring-border">
      <div className="flex h-44 items-end gap-[2px]">
        {points.map((p) => (
          <div key={p.date} className="group relative flex h-full flex-1 items-end" title={t('reports:insights.trendTitle', { date: label(p.date), time: fmt.watchTime(p.watched_ms), plays: t('reports:counts.plays', { count: p.plays }) })}>
            <div
              className="w-full rounded-t-[3px] bg-accent/80 transition-colors group-hover:bg-accent"
              style={{ height: p.watched_ms ? `${Math.max(2, (p.watched_ms / most) * 100)}%` : '0' }}
            />
          </div>
        ))}
      </div>
      <div className="mt-2 flex justify-between text-xs text-subtle">
        {ticks.map((i) => (
          <span key={i}>{points[i] && label(points[i].date)}</span>
        ))}
      </div>
    </div>
  );
}

function TopList({ titles, items, empty }: { titles: RankedTitle[]; items: StatsContext['items']; empty: string }) {
  const { t } = useTranslation('reports');
  if (!titles.length) return <p className="text-sm text-muted">{empty}</p>;
  return (
    <ol className="space-y-2">
      {titles.map((tabId, i) => {
        const item = items[tabId.id];
        const body = (
          <>
            <span className="w-5 text-right text-sm tabular-nums text-subtle">{i + 1}</span>
            <Artwork src={item?.images?.primary} alt="" className="w-10 shrink-0 rounded-md" icon={tabId.episodes ? 'tv' : 'film'} />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-[14px] font-medium">{tabId.title}</span>
              <span className="block text-xs text-muted">
                {[t('reports:counts.plays', { count: tabId.plays }), tabId.episodes && t('reports:counts.episodes', { count: tabId.episodes }), t('reports:counts.profiles', { count: tabId.profiles })].filter(Boolean).join(' · ')}
              </span>
            </span>
            <span className="text-sm tabular-nums text-muted">{fmt.watchTime(tabId.watched_ms)}</span>
          </>
        );
        const cls = 'flex items-center gap-3 rounded-xl p-1.5 pr-3';
        return (
          <li key={tabId.id}>
            {item ? (
              <Link to={itemHref(item)} className={cn(cls, 'transition-colors hover:bg-surface')}>
                {body}
              </Link>
            ) : (
              <div className={cls}>{body}</div>
            )}
          </li>
        );
      })}
    </ol>
  );
}

function Bars({ rows }: { rows: { key: string; label: ReactNode; bucket: Bucket }[] }) {
  const { t } = useTranslation('reports');
  if (!rows.length) return <p className="text-sm text-muted">{t("reports:insights.nothingYet")}</p>;
  const most = Math.max(1, ...rows.map((r) => r.bucket.watched_ms));
  return (
    <ul className="space-y-3">
      {rows.slice(0, 8).map((r) => (
        <li key={r.key}>
          <div className="mb-1 flex items-baseline justify-between gap-3 text-[13.5px]">
            <span className="min-w-0 truncate">{r.label}</span>
            <span className="shrink-0 text-xs tabular-nums text-muted">
              {fmt.watchTime(r.bucket.watched_ms)} · {t('reports:counts.plays', { count: r.bucket.plays })}
            </span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-surface-2">
            <div className="h-full rounded-full bg-accent" style={{ width: `${(r.bucket.watched_ms / most) * 100}%` }} />
          </div>
        </li>
      ))}
    </ul>
  );
}

// Monday first, as in the UK.
const weekdayOrder = [1, 2, 3, 4, 5, 6, 0];
function weekdayNames() { return Array.from({ length: 7 }, (_, day) => fmt.weekday(day)); }

function Heatmap({ hours }: { hours: number[][] }) {
  const { t } = useTranslation('reports');
  const most = Math.max(1, ...hours.flat());
  return (
    <div className="overflow-x-auto rounded-2xl bg-surface p-5 ring-1 ring-border">
      <div className="grid min-w-[400px] grid-cols-[2.25rem_repeat(24,minmax(0,1fr))] gap-[3px] text-[11px] text-subtle">
        {weekdayOrder.map((day) => (
          <div key={day} className="contents">
            <span className="self-center">{weekdayNames()[day]}</span>
            {hours[day].map((ms, h) => (
              <div
                key={h}
                className="aspect-square rounded-[3px] bg-accent"
                style={{ opacity: ms ? 0.15 + 0.85 * (ms / most) : 0.06 }}
                title={t('reports:insights.heatmapTitle', { day: weekdayNames()[day], hour: String(h).padStart(2, '0'), time: fmt.watchTime(ms) })}
              />
            ))}
          </div>
        ))}
        <span />
        {Array.from({ length: 24 }, (_, h) => (
          <span key={h} className="text-center">
            {h % 6 === 0 ? String(h).padStart(2, '0') : ''}
          </span>
        ))}
      </div>
    </div>
  );
}

// History

function HistoryList({ profile, library }: { profile: string; library?: number }) {
  const { t } = useTranslation('reports');
  const history = useInfiniteQuery({
    queryKey: ['stats-history', profile, library],
    queryFn: ({ pageParam }) => api.statsHistory({ profile, library, before: pageParam || undefined }),
    initialPageParam: 0,
    getNextPageParam: (last) => (last.more ? last.plays[last.plays.length - 1]?.id : undefined),
  });
  if (history.error) return <ErrorState error={history.error} />;
  if (!history.data) return <div className="h-96 animate-pulse rounded-2xl bg-surface" />;
  const pages = history.data.pages;
  const items: StatsContext['items'] = Object.assign({}, ...pages.map((p) => p.items));
  const profiles = pages[0].profiles;
  const plays = pages.flatMap((p) => p.plays);
  if (!plays.length) {
    return (
      <EmptyState icon={<History />} title={t("reports:insights.noWatchHistoryYet")}>{t("reports:insights.everyMovieAndEpisode")}</EmptyState>
    );
  }
  return (
    <div>
      <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
        {plays.map((p) => (
          <PlayRow key={p.id} play={p} items={items} profile={profiles.find((u) => u.id === p.user_id)} />
        ))}
      </div>
      {history.hasNextPage && (
        <div className="mt-5 flex justify-center">
          <Button variant="secondary" onClick={() => history.fetchNextPage()} disabled={history.isFetchingNextPage}>
            {history.isFetchingNextPage && <Loader2 className="animate-spin" />} {t("reports:insights.showMore")}</Button>
        </div>
      )}
    </div>
  );
}

function PlayRow({ play: p, items, profile }: { play: Play; items: StatsContext['items']; profile?: StatsContext['profiles'][number] }) {
  const { t } = useTranslation('reports');
  const item = items[p.item_id];
  const series = p.series_id ? items[p.series_id] : undefined;
  const link = series ? itemHref(series) : item ? itemHref(item) : undefined;
  const image = item?.images?.primary ?? series?.images?.backdrop;
  const pct = p.completed ? 100 : p.duration_ms ? Math.min(100, Math.round((p.position_ms / p.duration_ms) * 100)) : 0;
  return (
    <div className="flex items-center gap-4 px-4 py-3">
      <Artwork src={image} alt="" kind="still" className="hidden w-24 shrink-0 rounded-lg sm:block" icon={p.series_id ? 'tv' : 'film'} />
      <div className="min-w-0 flex-1">
        {link ? (
          <Link to={link} className="block truncate text-[14px] font-medium hover:underline">
            {p.series_title || p.title}
          </Link>
        ) : (
          <span className="block truncate text-[14px] font-medium">{p.series_title || p.title}</span>
        )}
        <p className="truncate text-[13px] text-muted">
          {[p.series_title && p.title, fmt.dateTime(new Date(p.started_at * 1000).toISOString()), p.device].filter(Boolean).join(' · ')}
        </p>
      </div>
      <span className="hidden items-center gap-2 text-[13px] md:flex">
        <Avatar name={profile?.name ?? '?'} color={profile?.color ?? 'violet'} className="size-6 text-[11px]" />
        {profile?.name ?? t("reports:insights.removedProfile")}
      </span>
      <span className="hidden w-24 text-right text-[13px] text-muted lg:block">{methodName(p.method)}</span>
      <div className="w-28 shrink-0 text-right">
        <p className="text-[13px] tabular-nums">{fmt.watchTime(p.watched_ms)}</p>
        <p className="flex items-center justify-end gap-1 text-xs text-subtle">
          {p.completed ? (
            <>
              <Check className="size-3 text-accent" strokeWidth={3} /> {t("reports:insights.finished")}</>
          ) : (
            t('reports:insights.percentThrough', { percent: pct })
          )}
        </p>
      </div>
    </div>
  );
}

// Storage

function Storage({ library }: { library?: number }) {
  const { t } = useTranslation('reports');
  const storage = useQuery({ queryKey: ['stats-storage', library], queryFn: () => api.statsStorage(library), placeholderData: keepPreviousData });
  const [list, setList] = useState<'biggest' | 'never'>('biggest');
  if (storage.error) return <ErrorState error={storage.error} />;
  const d = storage.data;
  if (!d) return <div className="h-96 animate-pulse rounded-2xl bg-surface" />;
  const libs = d.libraries.filter((l) => !library || l.id === library);
  if (!libs.some((l) => l.titles > 0)) {
    return (
      <EmptyState icon={<HardDrive />} title={t("reports:insights.nothingToMeasureYet")}>{t("reports:insights.onceYourLibrariesHave")}</EmptyState>
    );
  }
  const rows = list === 'biggest' ? d.biggest : d.never_watched;
  return (
    <div className="space-y-10">
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {libs.map((l) => {
          const unwatchedPct = l.size ? (l.unwatched_size / l.size) * 100 : 0;
          return (
            <div key={l.id} className="rounded-2xl bg-surface p-5 ring-1 ring-border">
              <div className="flex items-baseline justify-between gap-3">
                <p className="truncate font-semibold">{l.name}</p>
                <p className="text-2xl font-semibold tabular-nums tracking-tight">{fmt.fileSize(l.size)}</p>
              </div>
              <p className="mt-0.5 text-[13px] text-muted">
                {l.kind === 'shows' ? t('reports:counts.shows', { count: l.titles }) : l.kind === 'videos' ? t('reports:counts.videos', { count: l.titles }) : t('reports:counts.movies', { count: l.titles })} · {t('reports:counts.files', { count: l.files })}
              </p>
              <div className="mt-4 h-2 overflow-hidden rounded-full bg-accent">
                <div className="ml-auto h-full bg-warning/80" style={{ width: `${unwatchedPct}%` }} />
              </div>
              <p className="mt-2 text-[13px] text-muted">
                {l.unwatched_titles === 0 ? (
                  t("reports:insights.everythingHereHasBeen")
                ) : (
                  <>
                    <Trans ns="reports" i18nKey="insights.unwatchedStorage" count={l.unwatched_titles} values={{ size: fmt.fileSize(l.unwatched_size), percent: Math.round(unwatchedPct) }} components={{ size: <span className="text-foreground" /> }} />
                  </>
                )}
              </p>
            </div>
          );
        })}
      </div>

      <section>
        <div className="mb-4 flex flex-wrap items-center gap-1">
          {(
            [
              ['biggest', t('reports:insights.biggest')],
              ['never', t('reports:insights.never')],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              onClick={() => setList(id)}
              className={cn('rounded-full px-3.5 py-1 text-[13px] font-medium transition-colors', list === id ? 'bg-foreground text-background' : 'text-muted hover:text-foreground')}
            >
              {label}
            </button>
          ))}
          <span className="ml-3 text-[13px] text-subtle">
            {list === 'never' ? t("reports:insights.moviesAndShowsNo") : t("reports:insights.whatTakesUpThe")}
          </span>
        </div>
        {rows.length === 0 ? (
          <p className="text-sm text-muted">{t("reports:insights.everythingHereHasBeen")}</p>
        ) : (
          <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
            {rows.map((tabId) => (
              <SizeRow key={tabId.item_id} size={tabId} item={d.items[tabId.item_id]} />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function SizeRow({ size: titleInfo, item }: { size: TitleSize; item?: Item }) {
  const { t } = useTranslation('reports');
  if (!item) return null;
  return (
    <Link to={itemHref(item)} className="flex items-center gap-4 px-4 py-2.5 transition-colors hover:bg-surface-2/60">
      <Artwork src={item.images?.primary} alt="" className="w-9 shrink-0 rounded-md" icon={item.kind === 'series' ? 'tv' : 'film'} />
      <div className="min-w-0 flex-1">
        <p className="truncate text-[14px] font-medium">
          {item.title} {item.year ? <span className="font-normal text-muted">({item.year})</span> : null}
        </p>
        <p className="truncate text-[13px] text-muted">
          {[t('reports:counts.files', { count: titleInfo.files }), t('reports:insights.added', { time: fmt.ago(titleInfo.added_at) })].join(' · ')}
        </p>
      </div>
      <span className={cn('hidden text-[13px] sm:block', titleInfo.watched ? 'text-muted' : 'text-warning')}>
        {titleInfo.watched ? (titleInfo.last_played ? t('reports:insights.watchedAt', { time: fmt.ago(titleInfo.last_played) }) : t("reports:insights.watched")) : t("reports:insights.neverWatched")}
      </span>
      <span className="w-20 text-right text-[14px] font-medium tabular-nums">{fmt.fileSize(titleInfo.size)}</span>
    </Link>
  );
}

