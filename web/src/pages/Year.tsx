import i18n from '@/i18n';
import { Trans, useTranslation } from 'react-i18next';
import type { ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { motion } from 'motion/react';
import { Flame, Sparkles } from 'lucide-react';
import { api, itemHref, type RankedTitle, type YearReview } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { Artwork } from '@/components/Artwork';
import { EmptyState, ErrorState, Page } from '@/components/Page';

function monthNames() { return Array.from({ length: 12 }, (_, month) => fmt.month(month + 1)); }
function longMonths() { return Array.from({ length: 12 }, (_, month) => fmt.month(month + 1, 'long')); }
function weekdays() { return [i18n.t('reports:weekdays.sundays'), i18n.t('reports:weekdays.mondays'), i18n.t('reports:weekdays.tuesdays'), i18n.t('reports:weekdays.wednesdays'), i18n.t('reports:weekdays.thursdays'), i18n.t('reports:weekdays.fridays'), i18n.t('reports:weekdays.saturdays')]; }

/** The current profile's year in review: what they watched, how much, and when. */
export function YearPage() {
  const { t } = useTranslation('reports');
  const [params, setParams] = useSearchParams();
  const year = Number(params.get('year')) || undefined;
  const review = useQuery({ queryKey: ['my-year', year], queryFn: () => api.myYear(year), placeholderData: keepPreviousData });
  const { data: auth } = useAuth();
  if (review.error) return <ErrorState error={review.error} />;
  const y = review.data;

  return (
    <Page className="pb-20">
      <header className="flex flex-wrap items-end justify-between gap-4 px-6 pb-6 pt-10 md:px-10 md:pt-14">
        <div>
          <p className="flex items-center gap-2 text-sm font-medium text-accent">
            <Sparkles className="size-4" /> {auth?.profile?.name ? t('reports:year.profileYear', { name: auth.profile.name }) : t("reports:year.yourYear")}
          </p>
          <h1 className="mt-1 text-3xl font-bold md:text-4xl">{t('reports:year.reviewTitle', { year: y?.year ?? '' })}</h1>
        </div>
        {y && y.years.length > 1 && (
          <div className="flex flex-wrap gap-1">
            {y.years.map((n) => (
              <button
                key={n}
                onClick={() => setParams(n === new Date().getFullYear() ? {} : { year: String(n) }, { replace: true })}
                className={cn('rounded-full px-3.5 py-1 text-[13px] font-medium tabular-nums transition-colors', n === y.year ? 'bg-foreground text-background' : 'text-muted hover:text-foreground')}
              >
                {n}
              </button>
            ))}
          </div>
        )}
      </header>
      <div className="px-6 md:px-10">
        {!y ? (
          <div className="h-96 animate-pulse rounded-3xl bg-surface" />
        ) : y.plays === 0 ? (
          <EmptyState icon={<Sparkles />} title={t('reports:year.emptyYear', { year: y.year })}>{t("reports:year.asYouWatchMovies")}</EmptyState>
        ) : (
          <Review y={y} faded={review.isPlaceholderData} />
        )}
      </div>
    </Page>
  );
}

function Review({ y, faded }: { y: YearReview; faded: boolean }) {
  const { t } = useTranslation('reports');
  const hours = Math.round(y.watched_ms / 3_600_000);
  const busiestMonth = y.months.indexOf(Math.max(...y.months));
  const busiestDay = y.weekdays.indexOf(Math.max(...y.weekdays));
  const busiestHour = y.hours.indexOf(Math.max(...y.hours));
  const topGenre = y.genres[0];
  const firstItem = y.first && (y.items[y.first.series_id || y.first.item_id] ?? y.items[y.first.item_id]);
  const bingeItem = y.binge && y.items[y.binge.series_id];
  const introValues = {
    amount: hours < 1 ? fmt.watchTime(y.watched_ms) : fmt.number(hours),
    hourUnit: t('reports:year.hourNoun', { count: hours }),
    genre: topGenre?.name.toLowerCase() ?? '',
    extraDays: hours >= 24 ? t('reports:year.fullDays', { count: Math.round(hours / 24) }) : '',
  };
  const introComponents = {
    intro: <p className="text-sm font-medium text-white/75" />,
    amount: <p className="mt-1 text-6xl font-bold tracking-tight tabular-nums md:text-8xl" />,
    unit: <span className="ml-3 text-3xl font-semibold md:text-5xl" />,
    detail: <p className="mt-3 max-w-xl text-[15px] text-white/80" />,
  };

  return (
    <div className={cn('space-y-12 transition-opacity', faded && 'opacity-60')}>
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
        className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-zinc-700 via-zinc-800 to-zinc-950 p-8 text-white ring-1 ring-white/10 md:p-12"
      >
        {hours < 1 ? (
          topGenre
            ? <Trans ns="reports" i18nKey="year.introMinutesGenre" count={y.days} values={introValues} components={introComponents} />
            : <Trans ns="reports" i18nKey="year.introMinutes" count={y.days} values={introValues} components={introComponents} />
        ) : (
          topGenre
            ? <Trans ns="reports" i18nKey="year.introHoursGenre" count={y.days} values={introValues} components={introComponents} />
            : <Trans ns="reports" i18nKey="year.introHours" count={y.days} values={introValues} components={introComponents} />
        )}
        <div className="mt-8 grid grid-cols-2 gap-6 sm:grid-cols-4">
          <Stat value={y.movies} label={t('reports:year.movieNoun', { count: y.movies })} />
          <Stat value={y.shows} label={t('reports:year.showNoun', { count: y.shows })} />
          <Stat value={y.episodes} label={t('reports:year.episodeNoun', { count: y.episodes })} />
          <Stat value={y.streak} label={t('reports:year.streakNoun', { count: y.streak })} />
        </div>
      </motion.div>

      {y.top_movies.length > 0 && (
        <Block title={t("reports:year.yourTopMovies")}>
          <Posters titles={y.top_movies} y={y} />
        </Block>
      )}
      {y.top_shows.length > 0 && (
        <Block title={t("reports:year.yourTopShows")}>
          <Posters titles={y.top_shows} y={y} />
        </Block>
      )}

      <div className="grid gap-12 lg:grid-cols-2">
        <Block title={t("reports:year.whenYouWatch")}>
          <div className="rounded-2xl bg-surface p-5 ring-1 ring-border">
            <div className="flex h-36 items-end gap-1.5">
              {y.months.map((ms, i) => (
                <div key={i} className="flex h-full flex-1 flex-col items-center justify-end gap-1.5" title={t('reports:year.monthTime', { month: longMonths()[i], time: fmt.watchTime(ms) })}>
                  <div
                    className={cn('w-full rounded-t-md', i === busiestMonth ? 'bg-accent' : 'bg-accent/35')}
                    style={{ height: ms ? `${Math.max(3, (ms / Math.max(...y.months)) * 100)}%` : '0' }}
                  />
                  <span className="text-[11px] text-subtle">{monthNames()[i].charAt(0)}</span>
                </div>
              ))}
            </div>
            <p className="mt-4 text-[14px] text-foreground/85">
              <Trans ns="reports" i18nKey="year.peakTimes" values={{ month: longMonths()[busiestMonth], day: weekdays()[busiestDay], hour: hourName(busiestHour) }} components={{ month: <span className="font-semibold text-foreground" />, day: <span className="font-semibold text-foreground" />, hour: <span className="font-semibold text-foreground" /> }} />
            </p>
          </div>
        </Block>
        <div className="space-y-12">
          {y.binge && (
            <Block title={t("reports:year.yourBiggestBinge")}>
              <Moment item={bingeItem} icon={<Flame className="size-5 text-warning" />}><Trans ns="reports" i18nKey="year.binge" count={y.binge.episodes} values={{ date: fmt.date(y.binge.date), title: y.binge.title }} components={{ date: <span className="font-semibold" />, episodes: <span className="font-semibold" /> }} /></Moment>
            </Block>
          )}
          {y.first && (
            <Block title={t("reports:year.whereItStarted")}>
              <Moment item={firstItem}>{y.first.series_title ? <Trans ns="reports" i18nKey="year.firstEpisode" values={{ title: y.first.series_title, episode: y.first.title, date: fmt.date(new Date(y.first.started_at * 1000).toISOString().slice(0, 10)) }} components={{ title: <span className="font-semibold" /> }} /> : <Trans ns="reports" i18nKey="year.firstMovie" values={{ title: y.first.title, date: fmt.date(new Date(y.first.started_at * 1000).toISOString().slice(0, 10)) }} components={{ title: <span className="font-semibold" /> }} />}</Moment>
            </Block>
          )}
        </div>
      </div>

      {y.genres.length > 0 && (
        <Block title={t("reports:year.yourGenres")}>
          <div className="flex flex-wrap gap-2">
            {y.genres.map((g, i) => (
              <span key={g.name} className={cn('rounded-full px-4 py-2 text-sm ring-1', i === 0 ? 'bg-accent/15 text-foreground ring-accent/40' : 'bg-surface ring-border')}>
                {g.name} <span className="text-subtle">{Math.round((g.watched_ms / y.watched_ms) * 100)}%</span>
              </span>
            ))}
          </div>
        </Block>
      )}

      {y.people.length > 0 && (
        <Block title={t("reports:year.facesYouSawMost")}>
          <div className="flex flex-wrap gap-6">
            {y.people.map((p) => (
              <Link key={p.id} to={`/person/${p.id}`} className="group w-28 text-center">
                <div className="mx-auto size-24 overflow-hidden rounded-full bg-surface-2 ring-1 ring-border transition-transform duration-300 group-hover:scale-105">
                  {p.image ? <img src={p.image} alt="" className="size-full object-cover" /> : <span className="flex size-full items-center justify-center text-2xl font-semibold text-subtle">{p.name.charAt(0)}</span>}
                </div>
                <p className="mt-2 text-[13px] font-medium leading-tight">{p.name}</p>
                <p className="text-xs text-subtle">{t('reports:counts.titles', { count: p.titles })}</p>
              </Link>
            ))}
          </div>
        </Block>
      )}
    </div>
  );
}

function hourName(h: number) {
  if (h === 0) return i18n.t('reports:time.midnight');
  if (h === 12) return i18n.t('reports:time.midday');
  return h < 12 ? i18n.t('reports:time.am', { hour: h }) : i18n.t('reports:time.pm', { hour: h - 12 });
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <div>
      <p className="text-3xl font-semibold tabular-nums">{fmt.number(value)}</p>
      <p className="text-[13px] text-white/70">{label}</p>
    </div>
  );
}

function Block({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section>
      <h2 className="mb-4 text-xl font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function Posters({ titles, y }: { titles: RankedTitle[]; y: YearReview }) {
  const { t } = useTranslation('reports');
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-6 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
      {titles.map((tabId, i) => {
        const item = y.items[tabId.id];
        const poster = (
          <div className="relative">
            <Artwork src={item?.images?.primary} alt={tabId.title} fallbackTitle={tabId.title} icon={tabId.episodes ? 'tv' : 'film'} className="rounded-xl ring-1 ring-border" />
            <span className="absolute left-2 top-2 flex size-7 items-center justify-center rounded-full bg-black/70 text-sm font-semibold text-white backdrop-blur">{i + 1}</span>
          </div>
        );
        return (
          <div key={tabId.id}>
            {item ? (
              <Link to={itemHref(item)} className="block transition-transform duration-300 hover:-translate-y-1">
                {poster}
              </Link>
            ) : (
              poster
            )}
            <p className="mt-2 truncate text-[13.5px] font-medium">{tabId.title}</p>
            <p className="text-xs text-muted">
              {tabId.episodes ? t('reports:counts.episodes', { count: tabId.episodes }) : t('reports:counts.times', { count: tabId.plays })} · {fmt.watchTime(tabId.watched_ms)}
            </p>
          </div>
        );
      })}
    </div>
  );
}

function Moment({ item, icon, children }: { item?: YearReview['items'][string]; icon?: ReactNode; children: ReactNode }) {
  const image = item?.images?.backdrop ?? item?.images?.primary;
  const body = (
    <div className="relative isolate flex min-h-32 items-end overflow-hidden rounded-2xl bg-surface p-5 ring-1 ring-border">
      {image && <img src={image} alt="" className="absolute inset-0 -z-10 size-full object-cover opacity-40" />}
      <div className="absolute inset-0 -z-10 bg-gradient-to-t from-background via-background/70 to-transparent" />
      <p className="flex items-start gap-2 text-[15px] leading-snug">
        {icon}
        <span>{children}</span>
      </p>
    </div>
  );
  return item ? (
    <Link to={itemHref(item)} className="block transition-transform duration-300 hover:-translate-y-0.5">
      {body}
    </Link>
  ) : (
    body
  );
}
