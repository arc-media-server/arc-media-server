import { useState } from 'react';
import { useParams } from 'react-router';
import { CollectionRows } from '@/components/Collections';
import { AddToPlaylistButton } from '@/components/Playlists';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Loader2, Play, RefreshCw, RotateCcw } from 'lucide-react';
import { api, backdropUrl, posterUrl, type MediaFile } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Artwork } from '@/components/Artwork';
import { MoreMenu, useItemDialogs } from '@/components/ItemActions';
import { isLocalVideo } from '@/lib/kinds';
import { useIsAdmin } from '@/lib/auth';
import { Badge, Hero, MetaLine, TitleHeading } from '@/components/Hero';
import { ErrorState, Page } from '@/components/Page';
import { usePlayer } from '@/components/Player';
import { Button } from '@/components/ui/button';
import { WatchedButton } from '@/components/WatchedButton';
import { StarRating, WatchlistButton } from '@/components/Rating';
import { SimilarRow } from '@/components/Similar';
import { CastRow, CrewLine, detailParts } from '@/components/Credits';
import { ExtrasRow, TrailerButton } from '@/components/Extras';
import { cn } from '@/lib/utils';
import { progressOf } from '@/components/WatchState';

export function MoviePage() {
  const { id } = useParams();
  const { data: item, error } = useQuery({ queryKey: ['item', id], queryFn: () => api.item(id!) });
  const play = usePlayer();
  const qc = useQueryClient();
  const admin = useIsAdmin();
  const { open: openDialog, dialogs } = useItemDialogs(item);
  // The edition chosen on this page (for titles with several); undefined until one is picked.
  const [edition, setEdition] = useState<string>();
  const refresh = useMutation({
    mutationFn: () => api.refreshItem(id!),
    onSuccess: (updated) => {
      qc.setQueryData(['item', id], updated);
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['health'] });
      qc.invalidateQueries({ queryKey: ['health-summary'] });
    },
  });

  if (error) return <ErrorState error={error} />;
  if (!item) return <div className="h-[70vh] animate-pulse bg-surface" />;

  // The title's versions (several files of the same part), and the editions among them.
  const present = item.files?.filter((f) => !f.missing && f.part <= 1) ?? [];
  const editions = [...new Set(present.map((f) => f.edition ?? ''))];
  const currentEdition = editions.includes(edition ?? '') ? (edition ?? '') : (editions[0] ?? '');
  const file = present.find((f) => (f.edition ?? '') === currentEdition) ?? item.files?.find((f) => !f.missing) ?? item.files?.[0];
  const resolutions = [...new Set(present.map((f) => fmt.videoStream(f)).filter(Boolean).map((v) => fmt.resolution(v!.width, v!.height)))];
  const video = fmt.videoStream(file);
  const audio = fmt.audioStreams(file)[0];
  const playable = !!item.stream_url && item.files?.some((f) => !f.missing);
  const resumeAt = item.user_data?.position_ms ?? 0;
  const details = detailParts(item);
  // A home video: a frame of it rather than a poster, and the day it was made.
  const local = isLocalVideo(item);
  const playFrom = (startMS: number) =>
    play({
      title: item.title,
      subtitle: [item.year, currentEdition].filter(Boolean).join(' · ') || undefined,
      src: item.stream_url!,
      itemId: item.id,
      startMS,
      edition: currentEdition,
    });

  return (
    <Page className="pb-20">
      <Hero backdrop={backdropUrl(item)} className="min-h-[70vh]">
        <div className="flex min-h-[70vh] items-end gap-8 px-6 pb-12 pt-24 md:px-10">
          <Artwork
            src={posterUrl(item)}
            alt={item.title}
            fallbackTitle={item.title}
            eager
            kind={local ? 'still' : undefined}
            className={cn('hidden shrink-0 rounded-2xl shadow-2xl shadow-black/50 ring-1 ring-white/10 lg:block', local ? 'w-96' : 'w-52')}
          />
          <div className="max-w-2xl">
            <TitleHeading item={item} />
            <div className="mt-4 flex flex-wrap items-center gap-3">
              <MetaLine parts={[local && item.release_date ? fmt.date(item.release_date) : item.year, fmt.runtime(item.runtime_min || fmt.durationMinutes(file?.duration_ms)), details.genres, details.rating]} />
              {details.contentRating && <Badge>{details.contentRating}</Badge>}
              {resolutions.length > 1
                ? resolutions.map((res) => <Badge key={res}>{res}</Badge>)
                : video && <Badge>{fmt.resolution(video.width, video.height)}</Badge>}
              {audio?.channels && audio.channels > 2 && <Badge>{fmt.channels(audio.channels)}</Badge>}
            </div>
            {item.tagline && <p className="mt-5 text-[15px] italic text-foreground/70">{item.tagline}</p>}
            {item.overview && <p className={cn('text-[15px] leading-relaxed text-foreground/85', item.tagline ? 'mt-2' : 'mt-5')}>{item.overview}</p>}
            <CrewLine item={item} />
            <div className="mt-4">
              <StarRating key={item.id} item={item} />
            </div>
            {item.metadata_state === 'unmatched' && admin && (
              <p className="mt-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-warning">
                <AlertTriangle className="size-4" /> Arc couldn't identify this movie online.
                <button className="font-semibold underline-offset-4 hover:underline" onClick={() => openDialog('match')}>
                  Identify it…
                </button>
              </p>
            )}
            {editions.length > 1 && (
              <div className="mt-6 flex flex-wrap items-center gap-1.5" role="radiogroup" aria-label="Edition">
                {editions.map((e) => (
                  <button
                    key={e}
                    type="button"
                    role="radio"
                    aria-checked={e === currentEdition}
                    onClick={() => setEdition(e)}
                    className={cn(
                      'rounded-full px-3.5 py-1.5 text-[13px] font-medium ring-1 transition-colors',
                      e === currentEdition ? 'bg-foreground text-background ring-foreground' : 'bg-black/20 text-foreground/80 ring-white/15 hover:bg-black/30',
                    )}
                  >
                    {e || 'Standard Edition'}
                  </button>
                ))}
              </div>
            )}
            <div className={cn('flex flex-wrap gap-3', editions.length > 1 ? 'mt-4' : 'mt-7')}>
              <Button
                variant="primary"
                size="lg"
                disabled={!playable}
                onClick={() => playFrom(resumeAt)}
                className="relative overflow-hidden"
              >
                <Play className="fill-current" /> {!playable ? 'File missing' : resumeAt ? `Resume · ${fmt.clock(resumeAt)}` : 'Play'}
                {resumeAt > 0 && (
                  <span className="absolute inset-x-0 bottom-0 h-1 bg-background/15">
                    <span className="block h-full bg-accent-strong" style={{ width: `${progressOf(item) * 100}%` }} />
                  </span>
                )}
              </Button>
              {resumeAt > 0 && playable && (
                <Button variant="secondary" size="lg" className="size-12 rounded-2xl px-0" onClick={() => playFrom(0)} title="Play from the beginning" aria-label="Play from the beginning">
                  <RotateCcw />
                </Button>
              )}
              <TrailerButton extras={item.extras} title={item.title} />
              <WatchedButton itemId={item.id} watched={!!item.user_data?.played} />
              <WatchlistButton item={item} />
              {playable && <AddToPlaylistButton item={item} />}
              {admin && (
                <>
                  {!isLocalVideo(item) && (
                  <Button variant="secondary" size="lg" onClick={() => refresh.mutate()} disabled={refresh.isPending}>
                    {refresh.isPending ? <Loader2 className="animate-spin" /> : <RefreshCw />} Refresh
                  </Button>
                  )}
                  <MoreMenu item={item} onOpen={openDialog} />
                </>
              )}
            </div>
            {refresh.error && <p className="mt-3 text-sm text-warning">{refresh.error.message}</p>}
          </div>
        </div>
      </Hero>

      <CastRow credits={item.credits} />
      <CollectionRows item={item} />
      <SimilarRow item={item} />
      <ExtrasRow extras={item.extras} title={item.title} />
      {item.files?.map((f) => <FileDetails key={f.id} file={f} several={(item.files?.length ?? 0) > 1} />)}
      {dialogs}
    </Page>
  );
}

export function FileDetails({ file, several }: { file: MediaFile; several?: boolean }) {
  const video = fmt.videoStream(file);
  const audio = fmt.audioStreams(file);
  const subs = fmt.subtitleStreams(file);
  return (
    <section className="mx-6 mt-10 md:mx-10">
      <h2 className="mb-4 text-lg font-semibold">
        {several
          ? ['About This Version:', file.edition, video && fmt.resolution(video.width, video.height), file.part > 0 && `Part ${file.part}`]
              .filter(Boolean)
              .join(' ')
          : 'About This File'}
      </h2>
      <div className="grid gap-px overflow-hidden rounded-2xl bg-border ring-1 ring-border sm:grid-cols-2 lg:grid-cols-4">
        <Detail label="Video">
          {video ? `${fmt.resolution(video.width, video.height)} · ${fmt.codec(video.codec)}` : '—'}
          {video && <span className="block text-subtle">{video.width}×{video.height}</span>}
        </Detail>
        <Detail label="Audio">
          {audio.length === 0
            ? '—'
            : audio.map((a) => (
                <span key={a.index} className="block">
                  {[fmt.language(a.language), fmt.codec(a.codec), fmt.channels(a.channels)].filter(Boolean).join(' · ')}
                </span>
              ))}
        </Detail>
        <Detail label="Subtitles">
          {subs.length === 0
            ? 'None'
            : subs.map((s) => (
                <span key={s.index} className="block">
                  {[fmt.language(s.language) || 'Unknown', fmt.codec(s.codec), s.is_forced && 'Forced'].filter(Boolean).join(' · ')}
                </span>
              ))}
        </Detail>
        <Detail label="File">
          {[file.container?.toUpperCase(), fmt.fileSize(file.size)].filter(Boolean).join(' · ')}
          {file.missing && <span className="block text-warning">Missing from disk</span>}
        </Detail>
      </div>
      <p className="mt-3 break-all font-mono text-xs text-subtle">{file.path}</p>
      {file.probe_error && <p className="mt-2 text-xs text-warning">Couldn't read this file: {file.probe_error}</p>}
    </section>
  );
}

function Detail({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="bg-surface p-4">
      <p className="text-[11px] font-semibold uppercase tracking-wider text-subtle">{label}</p>
      <div className="mt-1.5 text-sm leading-relaxed">{children}</div>
    </div>
  );
}
