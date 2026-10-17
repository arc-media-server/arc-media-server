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

const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const longMonths = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const weekdays = ['Sundays', 'Mondays', 'Tuesdays', 'Wednesdays', 'Thursdays', 'Fridays', 'Saturdays'];

/** The current profile's year in review: what they watched, how much, and when. */
export function YearPage() {
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
            <Sparkles className="size-4" /> {auth?.profile?.name ? `${auth.profile.name}'s year` : 'Your year'}
          </p>
          <h1 className="mt-1 text-3xl font-bold md:text-4xl">{y?.year ?? ''} in Review</h1>
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
          <EmptyState icon={<Sparkles />} title={`Nothing watched in ${y.year} yet`}>
            As you watch movies and shows, your year comes together here: your favourites, your busiest times and your biggest binge.
          </EmptyState>
        ) : (
          <Review y={y} faded={review.isPlaceholderData} />
        )}
      </div>
    </Page>
  );
}

function Review({ y, faded }: { y: YearReview; faded: boolean }) {
  const hours = Math.round(y.watched_ms / 3_600_000);
  const busiestMonth = y.months.indexOf(Math.max(...y.months));
  const busiestDay = y.weekdays.indexOf(Math.max(...y.weekdays));
  const busiestHour = y.hours.indexOf(Math.max(...y.hours));
  const topGenre = y.genres[0];
  const firstItem = y.first && (y.items[y.first.series_id || y.first.item_id] ?? y.items[y.first.item_id]);
  const bingeItem = y.binge && y.items[y.binge.series_id];

  return (
    <div className={cn('space-y-12 transition-opacity', faded && 'opacity-60')}>
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
        className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-accent-strong via-accent-strong/85 to-indigo-950 p-8 text-white ring-1 ring-white/10 md:p-12"
      >
        <p className="text-sm font-medium text-white/75">This year you spent</p>
        <p className="mt-1 text-6xl font-bold tracking-tight tabular-nums md:text-8xl">
          {hours < 1 ? fmt.watchTime(y.watched_ms) : hours.toLocaleString('en-GB')}
          {hours >= 1 && <span className="ml-3 text-3xl font-semibold md:text-5xl">{hours === 1 ? 'hour' : 'hours'}</span>}
        </p>
        <p className="mt-3 max-w-xl text-[15px] text-white/80">
          watching on {fmt.plural(y.days, 'day')}{topGenre ? `, mostly ${topGenre.name.toLowerCase()}` : ''}.
          {hours >= 24 && ` That's ${Math.round(hours / 24)} full ${Math.round(hours / 24) === 1 ? 'day' : 'days'}.`}
        </p>
        <div className="mt-8 grid grid-cols-2 gap-6 sm:grid-cols-4">
          <Stat value={y.movies} label={y.movies === 1 ? 'movie' : 'movies'} />
          <Stat value={y.shows} label={y.shows === 1 ? 'show' : 'shows'} />
          <Stat value={y.episodes} label={y.episodes === 1 ? 'episode' : 'episodes'} />
          <Stat value={y.streak} label={y.streak === 1 ? 'day streak' : 'days in a row, at best'} />
        </div>
      </motion.div>

      {y.top_movies.length > 0 && (
        <Block title="Your top movies">
          <Posters titles={y.top_movies} y={y} />
        </Block>
      )}
      {y.top_shows.length > 0 && (
        <Block title="Your top shows">
          <Posters titles={y.top_shows} y={y} />
        </Block>
      )}

      <div className="grid gap-12 lg:grid-cols-2">
        <Block title="When you watch">
          <div className="rounded-2xl bg-surface p-5 ring-1 ring-border">
            <div className="flex h-36 items-end gap-1.5">
              {y.months.map((ms, i) => (
                <div key={i} className="flex h-full flex-1 flex-col items-center justify-end gap-1.5" title={`${longMonths[i]}: ${fmt.watchTime(ms)}`}>
                  <div
                    className={cn('w-full rounded-t-md', i === busiestMonth ? 'bg-accent' : 'bg-accent/35')}
                    style={{ height: ms ? `${Math.max(3, (ms / Math.max(...y.months)) * 100)}%` : '0' }}
                  />
                  <span className="text-[11px] text-subtle">{monthNames[i].charAt(0)}</span>
                </div>
              ))}
            </div>
            <p className="mt-4 text-[14px] text-foreground/85">
              <span className="font-semibold text-foreground">{longMonths[busiestMonth]}</span> was your biggest month. You watch most on{' '}
              <span className="font-semibold text-foreground">{weekdays[busiestDay]}</span>, usually around{' '}
              <span className="font-semibold text-foreground">{hourName(busiestHour)}</span>.
            </p>
          </div>
        </Block>
        <div className="space-y-12">
          {y.binge && (
            <Block title="Your biggest binge">
              <Moment item={bingeItem} icon={<Flame className="size-5 text-warning" />}>
                On <span className="font-semibold">{fmt.date(y.binge.date)}</span> you watched{' '}
                <span className="font-semibold">{y.binge.episodes} episodes</span> of {y.binge.title}.
              </Moment>
            </Block>
          )}
          {y.first && (
            <Block title="Where it started">
              <Moment item={firstItem}>
                Your year began with <span className="font-semibold">{y.first.series_title || y.first.title}</span>
                {y.first.series_title ? ` (${y.first.title})` : ''} on {fmt.date(new Date(y.first.started_at * 1000).toISOString().slice(0, 10))}.
              </Moment>
            </Block>
          )}
        </div>
      </div>

      {y.genres.length > 0 && (
        <Block title="Your genres">
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
        <Block title="Faces you saw most">
          <div className="flex flex-wrap gap-6">
            {y.people.map((p) => (
              <Link key={p.id} to={`/person/${p.id}`} className="group w-28 text-center">
                <div className="mx-auto size-24 overflow-hidden rounded-full bg-surface-2 ring-1 ring-border transition-transform duration-300 group-hover:scale-105">
                  {p.image ? <img src={p.image} alt="" className="size-full object-cover" /> : <span className="flex size-full items-center justify-center text-2xl font-semibold text-subtle">{p.name.charAt(0)}</span>}
                </div>
                <p className="mt-2 text-[13px] font-medium leading-tight">{p.name}</p>
                <p className="text-xs text-subtle">{fmt.plural(p.titles, 'title')}</p>
              </Link>
            ))}
          </div>
        </Block>
      )}
    </div>
  );
}

function hourName(h: number) {
  if (h === 0) return 'midnight';
  if (h === 12) return 'midday';
  return h < 12 ? `${h}am` : `${h - 12}pm`;
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <div>
      <p className="text-3xl font-semibold tabular-nums">{value.toLocaleString('en-GB')}</p>
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
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-6 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
      {titles.map((t, i) => {
        const item = y.items[t.id];
        const poster = (
          <div className="relative">
            <Artwork src={item?.images?.primary} alt={t.title} fallbackTitle={t.title} icon={t.episodes ? 'tv' : 'film'} className="rounded-xl ring-1 ring-border" />
            <span className="absolute left-2 top-2 flex size-7 items-center justify-center rounded-full bg-black/70 text-sm font-semibold text-white backdrop-blur">{i + 1}</span>
          </div>
        );
        return (
          <div key={t.id}>
            {item ? (
              <Link to={itemHref(item)} className="block transition-transform duration-300 hover:-translate-y-1">
                {poster}
              </Link>
            ) : (
              poster
            )}
            <p className="mt-2 truncate text-[13.5px] font-medium">{t.title}</p>
            <p className="text-xs text-muted">
              {t.episodes ? fmt.plural(t.episodes, 'episode') : fmt.plural(t.plays, 'time')} · {fmt.watchTime(t.watched_ms)}
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
