import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useParams } from 'react-router';
import { CollectionRows } from '@/components/Collections';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Play, RotateCcw } from 'lucide-react';
import { api } from '@/lib/api';
import { SourceFilesButton } from '@/components/SourceFormats';
import * as fmt from '@/lib/format';
import { ProviderCredit } from '@/components/ProviderCredits';
import { useLibraries } from '@/components/Sidebar';
import { MoreMenu, useAnalyzeItem, useItemDialogs } from '@/components/ItemActions';
import { isLocalVideo } from '@/lib/kinds';
import { useIsAdmin } from '@/lib/auth';
import { Badge, TitleHero, Synopsis, MetaLine, TitleHeading } from '@/components/Hero';
import { ErrorState, ErrorNotice, Page } from '@/components/Page';
import { usePlayer } from '@/components/Player';
import { PlaybackOptionsButton } from '@/components/PlaybackOptions';
import { Button } from '@/components/ui/button';
import { WatchedButton } from '@/components/WatchedButton';
import { StarRating, WatchlistButton } from '@/components/Rating';
import { SimilarRow } from '@/components/Similar';
import { CastRow, CrewLine, detailParts } from '@/components/Credits';
import { useThemeMusic } from '@/components/ThemeMusic';
import { ExtrasRow, TrailerButton } from '@/components/Extras';
import { cn } from '@/lib/utils';
import { progressOf } from '@/components/WatchState';

export function MoviePage() {
  const { t } = useTranslation('titles');
  const { id } = useParams();
  const { data: item, error } = useQuery({ queryKey: ['item', id], queryFn: () => api.item(id!) });
  const play = usePlayer();
  const qc = useQueryClient();
  const admin = useIsAdmin();
  const { open: openDialog, dialogs } = useItemDialogs(item);
  const libraries = useLibraries().data;
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
  const analysis = useAnalyzeItem(item);

  useThemeMusic(item?.theme_url);

  if (error) return <ErrorState error={error} />;
  if (!item) return <div className="h-[70vh] animate-pulse bg-surface" />;

  // The title's versions (several files of the same part), and the editions among them.
  const present = item.files?.filter((f) => !f.missing && f.part <= 1) ?? [];
  const editions = [...new Set(present.map((f) => f.edition ?? ''))];
  const currentEdition = editions.includes(edition ?? '') ? (edition ?? '') : (editions[0] ?? '');
  const file = present.find((f) => (f.edition ?? '') === currentEdition) ?? item.files?.find((f) => !f.missing) ?? item.files?.[0];
  const singleVersion = present.filter(f => (f.edition ?? '') === currentEdition).length <= 1;
  const video = fmt.videoStream(file);
  const tracks = fmt.audioStreams(file);
  const audio = tracks.find((s) => s.is_default) ?? tracks[0];
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
      <TitleHero item={item} landscape={local}>
        <TitleHeading item={item} detail />
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <MetaLine parts={[local && item.release_date ? fmt.date(item.release_date) : item.year, fmt.runtime(item.runtime_min || fmt.durationMinutes(file?.duration_ms)), details.genres, details.rating]} />
          {details.contentRating && <Badge>{details.contentRating}</Badge>}
          {singleVersion ? <>
            {video && <Badge>{fmt.resolution(video.width, video.height)}</Badge>}
            {fmt.sourceFormats(video).map((label) => <Badge key={label}>{label}</Badge>)}
            {audio && <Badge>{fmt.audioFormat(audio)}</Badge>}
          </> : <Badge>{t('titles:movie.versionCount', { count: present.filter(f => (f.edition ?? '') === currentEdition).length })}</Badge>}
        </div>
        <ErrorNotice error={refresh.error} onRetry={() => refresh.mutate()} />
        {analysis.notice}
        {item.metadata_state === 'unmatched' && admin && (
          <p className="mt-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-warning">
            <AlertTriangle className="size-4" /> {t("titles:movie.unmatched")}<button className="font-semibold underline-offset-4 hover:underline" onClick={() => openDialog('match')}>{t("titles:movie.identifyIt")}</button>
          </p>
        )}
        {editions.length > 1 && (
          <div className="mt-6 flex flex-wrap items-center gap-1.5" role="radiogroup" aria-label={t("titles:movie.edition")}>
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
                {e || t("titles:movie.standardEdition")}
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
            <Play className="fill-current" /> {!playable ? t("titles:movie.fileMissing") : resumeAt ? t('titles:movie.resumeAt', { time: fmt.clock(resumeAt) }) : t("titles:movie.play")}
            {resumeAt > 0 && (
              <span className="absolute inset-x-0 bottom-0 h-1 bg-background/15">
                <span className="block h-full bg-accent-strong" style={{ width: `${progressOf(item) * 100}%` }} />
              </span>
            )}
          </Button>
          {resumeAt > 0 && playable && (
            <Button variant="secondary" size="lg" className="size-12 rounded-2xl px-0" onClick={() => playFrom(0)} title={t("titles:movie.playFromTheBeginning")} aria-label={t("titles:movie.playFromTheBeginning")}>
              <RotateCcw />
            </Button>
          )}
          {playable && <PlaybackOptionsButton item={{ title: item.title, subtitle: [item.year, currentEdition].filter(Boolean).join(' · ') || undefined, itemId: item.id, src: item.stream_url!, startMS: resumeAt, edition: currentEdition }} />}
          <SourceFilesButton id={item.id} title={item.title} />
          <TrailerButton extras={item.extras} title={item.title} />
          <WatchedButton itemId={item.id} watched={!!item.user_data?.played} />
          <WatchlistButton item={item} />
          <MoreMenu item={item} admin={admin} playable={!!playable} edition={currentEdition} onOpen={openDialog} onRefresh={() => refresh.mutate()} refreshing={refresh.isPending} onAnalyze={analysis.analyze} analyzing={analysis.analyzing} />
        </div>
        <div className="mt-6 grid items-start gap-6 2xl:grid-cols-[minmax(0,1fr)_minmax(200px,0.6fr)]">
          {(item.tagline || item.overview) && <div className="min-w-0">
            {item.tagline && <p className="mb-2 text-[15px] italic text-foreground/70">{item.tagline}</p>}
            {item.overview && <Synopsis key={item.id} text={item.overview} />}
          </div>}
          <div className="min-w-0 [&>dl]:mt-0"><CrewLine item={item} /><div className="mt-4"><StarRating key={item.id} item={item} /></div></div>
        </div>
      </TitleHero>

      <CastRow credits={item.credits} />
      <CollectionRows item={item} />
      <SimilarRow item={item} />
      <ExtrasRow extras={item.extras} title={item.title} />
      <ProviderCredit item={item} library={libraries?.find((l) => l.id === item.library_id)} />
      {dialogs}
    </Page>
  );
}
