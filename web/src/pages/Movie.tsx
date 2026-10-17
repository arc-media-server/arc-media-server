import { useParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Loader2, Play, RefreshCw } from 'lucide-react';
import { api, backdropUrl, posterUrl, type MediaFile } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Artwork } from '@/components/Artwork';
import { MoreMenu, useItemDialogs } from '@/components/ItemActions';
import { useIsAdmin } from '@/lib/auth';
import { Badge, Hero, MetaLine } from '@/components/Hero';
import { ErrorState, Page } from '@/components/Page';
import { usePlayer } from '@/components/Player';
import { Button } from '@/components/ui/button';

export function MoviePage() {
  const { id } = useParams();
  const { data: item, error } = useQuery({ queryKey: ['item', id], queryFn: () => api.item(id!) });
  const play = usePlayer();
  const qc = useQueryClient();
  const admin = useIsAdmin();
  const { open: openDialog, dialogs } = useItemDialogs(item);
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

  const file = item.files?.find((f) => !f.missing) ?? item.files?.[0];
  const video = fmt.videoStream(file);
  const audio = fmt.audioStreams(file)[0];
  const playable = !!item.stream_url && item.files?.some((f) => !f.missing);

  return (
    <Page className="pb-20">
      <Hero backdrop={backdropUrl(item)} className="min-h-[70vh]">
        <div className="flex min-h-[70vh] items-end gap-8 px-6 pb-12 pt-24 md:px-10">
          <Artwork
            src={posterUrl(item)}
            alt={item.title}
            fallbackTitle={item.title}
            eager
            className="hidden w-52 shrink-0 rounded-2xl shadow-2xl shadow-black/50 ring-1 ring-white/10 lg:block"
          />
          <div className="max-w-2xl">
            <h1 className="text-4xl font-bold leading-[1.05] md:text-6xl">{item.title}</h1>
            <div className="mt-4 flex flex-wrap items-center gap-3">
              <MetaLine parts={[item.year, fmt.runtime(item.runtime_min || fmt.durationMinutes(file?.duration_ms))]} />
              {video && <Badge>{fmt.resolution(video.width, video.height)}</Badge>}
              {audio?.channels && audio.channels > 2 && <Badge>{fmt.channels(audio.channels)}</Badge>}
            </div>
            {item.overview && <p className="mt-5 text-[15px] leading-relaxed text-foreground/85">{item.overview}</p>}
            {item.metadata_state === 'unmatched' && admin && (
              <p className="mt-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-warning">
                <AlertTriangle className="size-4" /> Arc couldn't identify this movie online.
                <button className="font-semibold underline-offset-4 hover:underline" onClick={() => openDialog('match')}>
                  Identify it…
                </button>
              </p>
            )}
            <div className="mt-7 flex flex-wrap gap-3">
              <Button
                variant="primary"
                size="lg"
                disabled={!playable}
                onClick={() => play({ title: item.title, subtitle: item.year ? String(item.year) : undefined, src: item.stream_url! })}
              >
                <Play className="fill-current" /> {playable ? 'Play' : 'File missing'}
              </Button>
              {admin && (
                <>
                  <Button variant="secondary" size="lg" onClick={() => refresh.mutate()} disabled={refresh.isPending}>
                    {refresh.isPending ? <Loader2 className="animate-spin" /> : <RefreshCw />} Refresh
                  </Button>
                  <MoreMenu onOpen={openDialog} />
                </>
              )}
            </div>
            {refresh.error && <p className="mt-3 text-sm text-warning">{refresh.error.message}</p>}
          </div>
        </div>
      </Hero>

      {item.files?.map((f) => <FileDetails key={f.id} file={f} />)}
      {dialogs}
    </Page>
  );
}

export function FileDetails({ file }: { file: MediaFile }) {
  const video = fmt.videoStream(file);
  const audio = fmt.audioStreams(file);
  const subs = fmt.subtitleStreams(file);
  return (
    <section className="mx-6 mt-10 md:mx-10">
      <h2 className="mb-4 text-lg font-semibold">About This File</h2>
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
