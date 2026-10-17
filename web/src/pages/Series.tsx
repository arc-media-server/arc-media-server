import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { motion } from 'motion/react';
import { AlertTriangle, ChevronDown, GitCompareArrows, Eye, EyeOff, ListOrdered, Loader2, Play, RefreshCw } from 'lucide-react';
import { api, backdropUrl, posterUrl, type Ordering, type SeasonEpisode } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { Artwork } from '@/components/Artwork';
import { MoreMenu, useItemDialogs } from '@/components/ItemActions';
import { useIsAdmin } from '@/lib/auth';
import { useSpoilerGuard } from '@/lib/home';
import { Badge, Hero, MetaLine } from '@/components/Hero';
import { CastRow, CrewLine, detailParts } from '@/components/Credits';
import { ExtrasRow, TrailerButton } from '@/components/Extras';
import { CompareOrdersDialog } from '@/components/CompareOrders';
import { CollectionRows } from '@/components/Collections';
import { ErrorState, Page } from '@/components/Page';
import { usePlayer } from '@/components/Player';
import { Button } from '@/components/ui/button';
import { WatchedButton } from '@/components/WatchedButton';
import { episodeCode, ProgressBar, progressOf } from '@/components/WatchState';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const UNPLACED = -1;

export function SeriesPage() {
  const { id } = useParams();
  const qc = useQueryClient();
  const [showMissing, setShowMissing] = useState(false);
  const [season, setSeason] = useState<number | null>(null);
  const [comparing, setComparing] = useState(false);

  const orderings = useQuery({ queryKey: ['orderings', id], queryFn: () => api.orderings(id!) });
  const seasons = useQuery({
    queryKey: ['seasons', id, showMissing],
    queryFn: () => api.seasons(id!, { missing: showMissing }),
  });

  const setOrdering = useMutation({
    mutationFn: (orderingId: number) => api.setDisplayOrdering(id!, orderingId),
    onSuccess: (data) => {
      qc.setQueryData(['orderings', id], data);
      setSeason(null);
      return qc.invalidateQueries({ queryKey: ['seasons', id] });
    },
  });
  const refresh = useMutation({
    mutationFn: () => api.refreshSeries(id!),
    onSuccess: (data) => {
      qc.setQueryData(['orderings', id], data);
      return qc.invalidateQueries({ queryKey: ['seasons', id] });
    },
  });

  const data = seasons.data;
  const series = data?.series;
  const { open: openDialog, dialogs } = useItemDialogs(series);
  const admin = useIsAdmin();
  const tabs = useMemo(() => {
    if (!data) return [];
    const t = data.seasons.map((s) => ({ number: s.season_number, name: s.name, count: s.episodes.length }));
    if (data.unplaced.length) t.push({ number: UNPLACED, name: 'Not in This Order', count: data.unplaced.length });
    return t;
  }, [data]);

  // Default to the first season whenever the available seasons change.
  useEffect(() => {
    if (tabs.length && (season === null || !tabs.some((t) => t.number === season))) setSeason(tabs[0].number);
  }, [tabs, season]);

  if (seasons.error) return <ErrorState error={seasons.error} />;
  if (!data || !series) return <div className="h-[60vh] animate-pulse bg-surface" />;

  const episodes =
    season === UNPLACED ? data.unplaced : (data.seasons.find((s) => s.season_number === season)?.episodes ?? []);
  const available = data.seasons.reduce((n, s) => n + s.episodes.filter((e) => e.available).length, 0);
  const upNext = nextToPlay(data.seasons.flatMap((s) => s.episodes));
  const details = detailParts(series);
  const seasonEpisodes = season === UNPLACED ? [] : episodes.filter((e) => e.available);
  const seasonWatched = seasonEpisodes.length > 0 && seasonEpisodes.every((e) => e.user_data?.played);

  return (
    <Page className="pb-20">
      <Hero backdrop={backdropUrl(series)} className="min-h-[56vh]">
        <div className="flex min-h-[56vh] items-end gap-8 px-6 pb-10 pt-24 md:px-10">
          <Artwork
            src={posterUrl(series)}
            alt={series.title}
            fallbackTitle={series.title}
            icon="tv"
            eager
            className="hidden w-44 shrink-0 rounded-2xl shadow-2xl shadow-black/50 ring-1 ring-white/10 lg:block"
          />
          <div className="max-w-2xl">
            <h1 className="text-4xl font-bold leading-[1.05] md:text-6xl">{series.title}</h1>
            <div className="mt-4">
              <MetaLine
                parts={[
                  series.year,
                  fmt.plural(data.seasons.filter((s) => s.season_number > 0).length, 'Season'),
                  fmt.plural(available, 'Episode'),
                  details.genres,
                  details.rating,
                  details.contentRating && <Badge>{details.contentRating}</Badge>,
                ]}
              />
            </div>
            {series.overview && <p className="mt-4 line-clamp-4 text-[15px] leading-relaxed text-foreground/85">{series.overview}</p>}
            <CrewLine item={series} />
            {series.metadata_state === 'unmatched' && admin && (
              <p className="mt-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-warning">
                <AlertTriangle className="size-4" /> Arc couldn't identify this show online. Episodes are shown from their file names.
                <button className="font-semibold underline-offset-4 hover:underline" onClick={() => openDialog('match')}>
                  Identify it…
                </button>
              </p>
            )}
            <div className="mt-7 flex flex-wrap gap-3">
              {upNext && <PlayButton episode={upNext} seriesTitle={series.title} />}
              <TrailerButton extras={series.extras} title={series.title} />
              {available > 0 && <WatchedButton itemId={series.id} watched={series.unwatched_count === 0} />}
              {admin && (
                <>
                  <Button variant="secondary" size="lg" onClick={() => refresh.mutate()} disabled={refresh.isPending}>
                    {refresh.isPending ? <Loader2 className="animate-spin" /> : <RefreshCw />} Refresh
                  </Button>
                  <MoreMenu onOpen={openDialog} />
                </>
              )}
            </div>
          </div>
        </div>
      </Hero>

      <div className="sticky top-0 z-30 border-b border-border bg-background/80 backdrop-blur-2xl">
        <div className="flex flex-wrap items-center gap-3 px-6 py-3 md:px-10">
          <div className="scrollbar-none -mx-1 flex w-full gap-1 overflow-x-auto px-1 sm:w-auto sm:flex-1">
            {tabs.map((t) => (
              <button
                key={t.number}
                onClick={() => setSeason(t.number)}
                className={cn(
                  'relative shrink-0 rounded-full px-4 py-1.5 text-[13.5px] font-medium transition-colors',
                  season === t.number ? 'text-background' : 'text-muted hover:text-foreground',
                  t.number === UNPLACED && season !== t.number && 'text-warning',
                )}
              >
                {season === t.number && (
                  <motion.span
                    layoutId="season-pill"
                    className={cn('absolute inset-0 rounded-full', t.number === UNPLACED ? 'bg-warning' : 'bg-foreground')}
                    transition={{ type: 'spring', bounce: 0.15, duration: 0.45 }}
                  />
                )}
                <span className="relative">{t.name}</span>
              </button>
            ))}
          </div>
          {season !== null && season !== UNPLACED && seasonEpisodes.length > 0 && (
            <WatchedButton itemId={series.id} season={season} watched={seasonWatched} size="sm" variant="ghost" label />
          )}
          <Button variant="ghost" size="sm" onClick={() => setShowMissing((v) => !v)} title="Show episodes you don't have">
            {showMissing ? <EyeOff /> : <Eye />} {showMissing ? 'Hide Missing' : 'Show Missing'}
          </Button>
          {orderings.data && (
            <OrderingMenu
              orderings={orderings.data.orderings}
              selected={orderings.data.display_ordering_id}
              pending={setOrdering.isPending}
              onSelect={(o) => setOrdering.mutate(o)}
              onCompare={orderings.data.orderings.length > 1 ? () => setComparing(true) : undefined}
            />
          )}
        </div>
      </div>

      <div className="px-6 pt-6 md:px-10">
        {season === UNPLACED && (
          <p className="mb-5 max-w-2xl text-sm text-muted">
            You have these episodes, but <span className="text-foreground">{data.ordering?.name}</span> doesn't include them.
            They're still here so nothing goes missing.
          </p>
        )}
        <motion.div
          key={`${season}-${data.ordering?.id}`}
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
          className="divide-y divide-border"
        >
          {episodes.map((ep) => (
            <EpisodeRow key={`${ep.id}-${ep.episode_number}`} episode={ep} seriesTitle={series.title} unplaced={season === UNPLACED} />
          ))}
        </motion.div>
      </div>
      <CastRow credits={series.credits} />
      <CollectionRows item={series} />
      <ExtrasRow extras={series.extras} title={series.title} />
      {dialogs}
      {orderings.data && (
        <CompareOrdersDialog
          seriesId={id!}
          orderings={orderings.data.orderings}
          current={orderings.data.display_ordering_id}
          open={comparing}
          onOpenChange={setComparing}
          onUse={(o) => setOrdering.mutate(o)}
        />
      )}
    </Page>
  );
}

/**
 * The episode the Play button starts: the one most recently watched if it's part-way through,
 * otherwise the next unwatched one after it, otherwise the first. Specials only if there's
 * nothing else.
 */
function nextToPlay(all: SeasonEpisode[]): SeasonEpisode | undefined {
  const eps = all.filter((e) => e.available && e.stream_url);
  const main = eps.filter((e) => e.season_number > 0);
  const list = main.length ? main : eps;
  let last = -1;
  list.forEach((e, i) => {
    if ((e.user_data?.last_played_at ?? 0) > (list[last]?.user_data?.last_played_at ?? 0)) last = i;
  });
  if (last < 0) return list[0];
  if (list[last].user_data?.position_ms) return list[last];
  return list.slice(last + 1).find((e) => !e.user_data?.played) ?? list.find((e) => !e.user_data?.played) ?? list[0];
}

function playEpisode(play: ReturnType<typeof usePlayer>, ep: SeasonEpisode, seriesTitle: string, fromStart = false) {
  play({
    title: ep.title,
    subtitle: `${seriesTitle} · ${episodeCode(ep.season_number, ep.episode_number)}`,
    src: ep.stream_url!,
    itemId: ep.id,
    startMS: fromStart ? 0 : ep.user_data?.position_ms,
  });
}

function PlayButton({ episode, seriesTitle }: { episode: SeasonEpisode; seriesTitle: string }) {
  const play = usePlayer();
  const resume = episode.user_data?.position_ms;
  return (
    <Button variant="primary" size="lg" onClick={() => playEpisode(play, episode, seriesTitle)}>
      <Play className="fill-current" /> {resume ? 'Resume' : 'Play'} {episodeCode(episode.season_number, episode.episode_number)}
    </Button>
  );
}

function OrderingMenu({
  orderings,
  selected,
  pending,
  onSelect,
  onCompare,
}: {
  orderings: Ordering[];
  selected: number;
  pending: boolean;
  onSelect: (id: number) => void;
  onCompare?: () => void;
}) {
  const current = orderings.find((o) => o.id === selected);
  const groups: [string, Ordering[]][] = [
    ['TheTVDB', orderings.filter((o) => o.source.startsWith('tvdb'))],
    ['TMDB', orderings.filter((o) => o.source.startsWith('tmdb'))],
    ['Custom', orderings.filter((o) => o.source === 'custom')],
    ['From File Names', orderings.filter((o) => o.source === 'file')],
  ];
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="secondary" size="sm" disabled={pending}>
          {pending ? <Loader2 className="animate-spin" /> : <ListOrdered />}
          {pending ? 'Matching episodes…' : (current?.name ?? 'Episode Order')}
          {current && !pending && <span className="text-subtle">{providerLabel(current)}</span>}
          <ChevronDown className="text-subtle" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <DropdownMenuRadioGroup value={String(selected)} onValueChange={(v) => onSelect(Number(v))}>
          {groups
            .filter(([, list]) => list.length)
            .map(([label, list], i) => (
              <div key={label}>
                {i > 0 && <DropdownMenuSeparator />}
                <DropdownMenuLabel>{label}</DropdownMenuLabel>
                {list.map((o) => (
                  <DropdownMenuRadioItem key={o.id} value={String(o.id)}>
                    <div className="min-w-0">
                      <p className="truncate">{o.name}</p>
                      <p className="text-xs text-subtle">
                        {[o.description?.replace(/^(TVDB|TMDB)( · )?/, ''), fmt.plural(o.episode_count, 'episode')].filter(Boolean).join(' · ')}
                      </p>
                    </div>
                  </DropdownMenuRadioItem>
                ))}
              </div>
            ))}
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        {onCompare && (
          <DropdownMenuItem onSelect={onCompare}>
            <GitCompareArrows className="size-4 text-muted" /> Compare Orders…
          </DropdownMenuItem>
        )}
        <DropdownMenuItem disabled className="text-xs text-subtle">
          Changes how episodes are shown. Your files aren't touched.
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function providerLabel(o: Ordering) {
  if (o.source.startsWith('tvdb')) return 'TVDB';
  if (o.source.startsWith('tmdb')) return 'TMDB';
  return '';
}

function EpisodeRow({ episode: ep, seriesTitle, unplaced }: { episode: SeasonEpisode; seriesTitle: string; unplaced: boolean }) {
  const play = usePlayer();
  const playable = ep.available && ep.stream_url;
  const onPlay = () => playable && playEpisode(play, ep, seriesTitle);
  const watched = !!ep.user_data?.played;
  const hideSpoilers = useSpoilerGuard()(ep);

  return (
    <div
      className={cn('group flex gap-5 py-4', playable ? 'cursor-pointer' : 'opacity-45')}
      onClick={onPlay}
      role={playable ? 'button' : undefined}
      tabIndex={playable ? 0 : undefined}
      onKeyDown={(e) => e.key === 'Enter' && onPlay()}
    >
      <div className="relative w-36 shrink-0 overflow-hidden rounded-xl ring-1 ring-border sm:w-56">
        <Artwork src={ep.images?.primary} alt={ep.title} kind="still" icon="tv" className={cn(hideSpoilers && 'scale-110 blur-xl')} />
        {playable && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/0 transition-colors duration-300 group-hover:bg-black/35">
            <div className="flex size-11 scale-90 items-center justify-center rounded-full bg-white/90 text-black opacity-0 shadow-xl transition-all duration-300 group-hover:scale-100 group-hover:opacity-100">
              <Play className="ml-0.5 size-5 fill-current" />
            </div>
          </div>
        )}
        <ProgressBar value={progressOf(ep)} />
      </div>
      <div className="min-w-0 flex-1 pt-0.5">
        <div className="flex items-baseline gap-3">
          {!unplaced && <span className="w-7 shrink-0 text-sm tabular-nums text-subtle">{ep.episode_number}</span>}
          <div className="min-w-0">
            <h3 className="truncate text-[15px] font-semibold">{ep.title}</h3>
            <div className="mt-0.5 flex flex-wrap gap-x-2 text-xs text-subtle">
              {[fmt.date(ep.release_date), fmt.runtime(ep.runtime_min), !ep.available && 'Not in library', ep.metadata_state === 'unmatched' && 'Not found online', ep.locked && 'Placed manually']
                .filter(Boolean)
                .map((p, i) => (
                  <span key={i}>{p}</span>
                ))}
            </div>
            {ep.overview && !hideSpoilers && <p className="mt-2 line-clamp-2 max-w-3xl text-sm leading-relaxed text-muted">{ep.overview}</p>}
            {ep.overview && hideSpoilers && <p className="mt-2 text-sm italic text-subtle">Description hidden until you watch it.</p>}
          </div>
        </div>
      </div>
      {ep.available && (
        <WatchedButton
          itemId={ep.id}
          watched={watched}
          size="sm"
          variant="ghost"
          className={cn('mt-0.5 shrink-0 transition-opacity', !watched && 'opacity-0 focus-visible:opacity-100 group-hover:opacity-100')}
        />
      )}
    </div>
  );
}
