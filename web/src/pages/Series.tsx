import { SourceFilesButton } from '@/components/SourceFormats';
import { EpisodeDetailsButton } from '@/components/EpisodeDetails';
import { QueueActions } from '@/components/QueueActions';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { motion } from 'motion/react';
import { AlertTriangle, Eye, EyeOff, ListOrdered, MoreHorizontal, Pencil, Play, Shuffle } from 'lucide-react';
import { api, type SeasonEpisode } from '@/lib/api';
import { EditDetailsDialog } from '@/components/EditDetails';
import { EditSeasonDialog } from '@/components/EditSeason';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { episodeQueue, shuffled } from '@/lib/shuffle';
import { Artwork } from '@/components/Artwork';
import { MoreMenu, useAnalyzeItem, useItemDialogs } from '@/components/ItemActions';
import { useIsAdmin } from '@/lib/auth';
import { useSpoilerGuard } from '@/lib/home';
import { Badge, TitleHero, Synopsis, MetaLine, TitleHeading } from '@/components/Hero';
import { CastRow, CrewLine, detailParts } from '@/components/Credits';
import { useThemeMusic } from '@/components/ThemeMusic';
import { ExtrasRow, TrailerButton } from '@/components/Extras';
import { CollectionRows } from '@/components/Collections';
import { useLibraries } from '@/components/Sidebar';
import { AddToPlaylistButton } from '@/components/Playlists';
import { ErrorState, ErrorNotice, Page } from '@/components/Page';
import { nowPlayingFor, usePlayer } from '@/components/Player';
import { PlaybackOptionsButton } from '@/components/PlaybackOptions';
import { Button } from '@/components/ui/button';
import { WatchedButton } from '@/components/WatchedButton';
import { StarRating, WatchlistButton } from '@/components/Rating';
import { SimilarRow } from '@/components/Similar';
import { episodeCode, ProgressBar, progressOf } from '@/components/WatchState';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const UNPLACED = -1;

export function SeriesPage() {
  const { id } = useParams();
  const qc = useQueryClient();
  const [showMissing, setShowMissing] = useState(false);
  const [season, setSeason] = useState<number | null>(null);
  const [editingEpisode, setEditingEpisode] = useState<SeasonEpisode | null>(null);
  const [editingSeason, setEditingSeason] = useState(false);
  const scanning = useLibraries().data?.some((library) => library.scan.running) ?? false;
  const wasScanning = useRef(scanning);

  useEffect(() => {
    if (wasScanning.current && !scanning) {
      void qc.invalidateQueries({ queryKey: ['orderings', id] });
      void qc.invalidateQueries({ queryKey: ['seasons', id] });
    }
    wasScanning.current = scanning;
  }, [scanning, id, qc]);

  const orderings = useQuery({ queryKey: ['orderings', id], queryFn: () => api.orderings(id!), refetchInterval: scanning ? 3000 : false });
  const seasons = useQuery({
    queryKey: ['seasons', id, showMissing],
    queryFn: () => api.seasons(id!, { missing: showMissing }),
    refetchInterval: scanning ? 3000 : false,
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
  const analysis = useAnalyzeItem(series);
  const { open: openDialog, dialogs } = useItemDialogs(series);
  const admin = useIsAdmin();
  const play = usePlayer();
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

  useThemeMusic(series?.theme_url);

  if (seasons.error) return <ErrorState error={seasons.error} />;
  if (!data || !series) return <div className="h-[60vh] animate-pulse bg-surface" />;

  const episodes =
    season === UNPLACED ? data.unplaced : (data.seasons.find((s) => s.season_number === season)?.episodes ?? []);
  const available = data.seasons.reduce((n, s) => n + s.episodes.filter((e) => e.available).length, 0);
  const upNext = nextToPlay(data.seasons.flatMap((s) => s.episodes));
  const details = detailParts(series);
  const seasonEpisodes = season === UNPLACED ? [] : episodes.filter((e) => e.available);
  const currentSeason = season === UNPLACED ? undefined : data.seasons.find((s) => s.season_number === season);
  const aired = data.ordering?.source === 'tmdb_aired' || data.ordering?.source === 'tvdb_aired';
  const seasonWatched = seasonEpisodes.length > 0 && seasonEpisodes.every((e) => e.user_data?.played);

  return (
    <Page className="pb-20">
      <TitleHero item={series}>
        <TitleHeading item={series} detail />
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
          {upNext && <PlaybackOptionsButton item={nowPlayingFor({ ...upNext, series_title: series.title })} />}
          {available > 1 && (
            <Button
              variant="secondary"
              size="lg"
              className="size-12 rounded-2xl px-0"
              title="Shuffle the episodes"
              aria-label="Shuffle"
              onClick={() => {
                const q = shuffled(episodeQueue(data.seasons.filter((s) => s.season_number !== 0).flatMap((s) => s.episodes), series.title));
                if (q.length) play(q[0], q);
              }}
            >
              <Shuffle />
            </Button>
          )}
          <TrailerButton extras={series.extras} title={series.title} />
          {available > 0 && <WatchedButton itemId={series.id} watched={series.unwatched_count === 0} />}
          <WatchlistButton item={series} />
          {available > 0 && <QueueActions item={series} />}
          {available > 0 && <AddToPlaylistButton item={series} />}
          {admin && (
            <>
              <MoreMenu item={series} onOpen={openDialog} onRefresh={() => refresh.mutate()} refreshing={refresh.isPending} onAnalyze={analysis.analyze} analyzing={analysis.analyzing} />
            </>
          )}
        </div>
        <div className="mt-6 grid items-start gap-6 2xl:grid-cols-[minmax(0,1fr)_minmax(200px,0.6fr)]">
          {series.overview && <Synopsis key={series.id} text={series.overview} />}
          <div className="min-w-0 [&>dl]:mt-0"><CrewLine item={series} /><div className="mt-4"><StarRating key={series.id} item={series} /></div></div>
        </div>
      </TitleHero>

      <div className="space-y-2 px-6 md:px-10">
        <ErrorNotice error={refresh.error} onRetry={() => refresh.mutate()} />
        {analysis.notice}
        <ErrorNotice error={orderings.error} onRetry={() => void orderings.refetch()} />
      </div>
      <CastRow credits={series.credits} />

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
          {seasonEpisodes.length > 1 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                const q = shuffled(episodeQueue(seasonEpisodes, series.title));
                if (q.length) play(q[0], q);
              }}
            >
              <Shuffle /> Shuffle Season
            </Button>
          )}
          {season !== null && season !== UNPLACED && seasonEpisodes.length > 0 && (
            <WatchedButton itemId={series.id} season={season} watched={seasonWatched} size="sm" variant="ghost" label />
          )}
          {admin && currentSeason && (
            <Button variant="ghost" size="sm" onClick={() => setEditingSeason(true)}>
              <Pencil /> Edit Season
            </Button>
          )}
          <Button variant="ghost" size="sm" onClick={() => setShowMissing((v) => !v)} title="Show episodes you don't have">
            {showMissing ? <EyeOff /> : <Eye />} {showMissing ? 'Hide Missing' : 'Show Missing'}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => openDialog('ordering')}><ListOrdered /> Episode ordering…</Button>
        </div>
      </div>

      <div className="px-6 pt-6 md:px-10">
        {currentSeason && (currentSeason.poster || currentSeason.overview) && (
          <div className="mb-6 flex items-start gap-5">
            {currentSeason.poster && (
              <Artwork src={currentSeason.poster} alt={currentSeason.name} icon="tv" className="w-24 shrink-0 rounded-xl shadow-lg ring-1 ring-border sm:w-28" />
            )}
            <div className="min-w-0 max-w-3xl pt-1">
              <h2 className="text-lg font-semibold">{currentSeason.name}</h2>
              <p className="mt-0.5 text-xs text-subtle">{fmt.plural(currentSeason.episodes.filter((e) => e.available).length, 'episode')}</p>
              {currentSeason.overview && <p className="mt-2 line-clamp-5 text-sm leading-relaxed text-muted">{currentSeason.overview}</p>}
            </div>
          </div>
        )}
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
            <EpisodeRow
              key={`${ep.id}-${ep.episode_number}`}
              episode={ep}
              seriesTitle={series.title}
              unplaced={season === UNPLACED}
              onEdit={admin ? () => setEditingEpisode(ep) : undefined}
            />
          ))}
        </motion.div>
      </div>
      <CollectionRows item={series} />
      <SimilarRow item={series} />
      <ExtrasRow extras={series.extras} title={series.title} />
      {dialogs}
      {editingEpisode && (
        <EditDetailsDialog item={editingEpisode} open onOpenChange={(o) => !o && setEditingEpisode(null)} />
      )}
      {currentSeason && (
        <EditSeasonDialog
          series={series}
          season={currentSeason}
          orderingId={data.ordering?.id}
          aired={aired}
          open={editingSeason}
          onOpenChange={setEditingSeason}
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
  const main = eps.filter((e) => e.season_number > 0 || e.inline_special);
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
  play(nowPlayingFor({ ...ep, series_title: seriesTitle }, fromStart));
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

function EpisodeRow({
  episode: ep,
  seriesTitle,
  unplaced,
  onEdit,
}: {
  episode: SeasonEpisode;
  seriesTitle: string;
  unplaced: boolean;
  onEdit?: () => void;
}) {
  const play = usePlayer();
  const playable = ep.available && ep.stream_url;
  const onPlay = () => playable && playEpisode(play, ep, seriesTitle);
  const watched = !!ep.user_data?.played;
  const hideSpoilers = useSpoilerGuard()(ep);

  return (
    <div
      className={cn('group flex flex-col gap-3 py-4 xl:flex-row xl:gap-5', !playable && 'opacity-45')}
    >
      <div className="relative grid min-w-0 flex-1 grid-cols-[96px_minmax(0,1fr)] gap-3 sm:flex sm:gap-5">
      <div className="relative w-24 shrink-0 overflow-hidden rounded-xl ring-1 ring-border sm:w-56">
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
          {!unplaced && <span className={cn('shrink-0 text-sm tabular-nums text-subtle', ep.inline_special ? 'w-auto' : 'w-7')}>{ep.inline_special ? episodeCode(0, ep.episode_number) : ep.episode_number}</span>}
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
      {playable && <button type="button" aria-label={`Play ${ep.title}`} onClick={onPlay}
        className="absolute inset-0 cursor-pointer rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-accent" />}
      </div>
      <div className="flex max-w-full flex-none flex-wrap content-start items-center justify-end gap-2 self-end xl:self-start xl:flex-nowrap" onClick={e => e.stopPropagation()} onKeyDown={e => e.stopPropagation()}>
      <EpisodeDetailsButton episode={ep} />
      {ep.available && <SourceFilesButton id={ep.id} title={ep.title} />}
      {playable && <PlaybackOptionsButton compact item={nowPlayingFor({ ...ep, series_title: seriesTitle })} />}
      {ep.available && <QueueActions item={{ ...ep, series_title: seriesTitle }} compact />}
      {ep.available && (
        <AddToPlaylistButton
          item={{ ...ep, series_title: seriesTitle }}
          size="icon"
          className="mt-0.5 size-8 shrink-0 bg-transparent opacity-100 md:opacity-0 transition-opacity hover:bg-surface-2 focus-visible:opacity-100 group-hover:opacity-100 [&_svg]:size-4"
        />
      )}
      {ep.available && (
        <WatchedButton
          itemId={ep.id}
          watched={watched}
          size="sm"
          variant="ghost"
          className={cn('mt-0.5 shrink-0 transition-opacity', !watched && 'opacity-100 md:opacity-0 focus-visible:opacity-100 group-hover:opacity-100')}
        />
      )}
      {onEdit && (
        <div onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={`More for ${ep.title}`}
                className="mt-0.5 size-8 shrink-0 opacity-100 md:opacity-0 transition-opacity focus-visible:opacity-100 group-hover:opacity-100 data-[state=open]:opacity-100 [&_svg]:size-4"
              >
                <MoreHorizontal />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={onEdit}>
                <Pencil className="size-4 text-muted" /> Edit…
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}
      </div>
    </div>
  );
}
