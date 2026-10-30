import { useTranslation } from 'react-i18next';
import { useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, ChevronRight, Copy } from 'lucide-react';
import { api, type MediaFile, type Stream } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { ErrorNotice } from '@/components/Page';

/**
 * Media Info, as Plex's: each source file's video, audio, subtitle and file details, opened from a
 * title's or an episode's "⋯" menu. The file details are read only on demand.
 */
export function MediaInfoDialog({ id, title, open, onOpenChange }: { id: string; title: string; open: boolean; onOpenChange: (o: boolean) => void }) {
  const { t } = useTranslation('titles');
  const query = useQuery({ queryKey: ['item', id], queryFn: () => api.item(id), enabled: open });
  const files = query.data?.files ?? [];
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent title={t('titles:mediaInfo.title')} description={title} className="w-[min(620px,calc(100vw-32px))]">
      <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />
      {query.isPending && <p role="status" className="text-sm text-muted">{t('titles:mediaInfo.loading')}</p>}
      {query.data && !files.length && <p className="text-sm text-muted">{t('titles:mediaInfo.noFiles')}</p>}
      {files.map(f => <FileDetails key={f.id} file={f} several={files.length > 1} />)}
      {files.length > 0 && <p className="pb-3 text-xs text-subtle">{t('titles:mediaInfo.sourceDisclaimer')}</p>}
    </DialogContent>
  </Dialog>;
}

function FileDetails({ file, several }: { file: MediaFile; several: boolean }) {
  const { t } = useTranslation('titles');
  const video = fmt.videoStream(file);
  const audio = fmt.audioStreams(file);
  const subs = fmt.subtitleStreams(file);
  const rate = (value?: number) => !value || value <= 0 ? '' : value >= 1e6
    ? t('titles:mediaInfo.megabits', { value: fmt.fixed(value / 1e6, 1) })
    : t('titles:mediaInfo.kilobits', { value: fmt.fixed(value / 1e3, 0) });
  const heading = [file.edition, video && fmt.resolution(video.width, video.height), file.part > 0 && t('titles:mediaInfo.part', { number: file.part })].filter(Boolean).join(' · ');

  return (
    <section className="mb-6 space-y-4">
      {several && <h3 className="text-[15px] font-semibold">{heading || t('titles:mediaInfo.version')}</h3>}
      {file.missing && <p className="text-sm text-warning">{t('titles:mediaInfo.missingFromDisk')}</p>}
      {file.probe_error && <p className="text-sm text-warning">{t('titles:mediaInfo.probeError', { error: file.probe_error })}</p>}
      {video && <Fields label={t('titles:mediaInfo.video')} rows={[
        [t('titles:mediaInfo.codec'), fmt.codec(video.codec)],
        [t('titles:mediaInfo.resolution'), video.width && video.height ? [`${video.width}×${video.height}`, fmt.resolution(video.width, video.height)].join(' · ') : ''],
        [t('titles:mediaInfo.aspectRatio'), fmt.aspectRatio(video.aspect_ratio)],
        [t('titles:mediaInfo.frameRate'), video.frame_rate ? t('titles:mediaInfo.fps', { value: fmt.number(video.frame_rate, { maximumFractionDigits: 3 }) }) : ''],
        [t('titles:mediaInfo.bitDepth'), video.bit_depth ? t('titles:mediaInfo.bits', { count: video.bit_depth }) : ''],
        [t('titles:mediaInfo.hdr'), hdr(video, t('titles:mediaInfo.sdr'), t('titles:mediaInfo.dvProfile', { profile: video.dolby_vision_profile }))],
        [t('titles:mediaInfo.bitrate'), rate(video.bitrate)],
      ]} />}
      {audio.length > 0 && <Tracks label={t('titles:mediaInfo.audio')}
        columns={[t('titles:mediaInfo.language'), t('titles:mediaInfo.codec'), t('titles:mediaInfo.channels'), t('titles:mediaInfo.bitrate')]}
        rows={audio.map(a => [fmt.language(a.language) || t('titles:mediaInfo.unknown'), fmt.codec(a.codec, a.profile), fmt.channels(a.channels), rate(a.bitrate)])} />}
      {subs.length > 0 && <Tracks label={t('titles:mediaInfo.subtitles')}
        columns={[t('titles:mediaInfo.language'), t('titles:mediaInfo.format'), t('titles:mediaInfo.forced')]}
        rows={subs.map(s => [fmt.language(s.language) || t('titles:mediaInfo.unknown'), fmt.codec(s.codec), s.is_forced ? t('titles:mediaInfo.yes') : t('titles:mediaInfo.no')])} />}
      <Fields label={t('titles:mediaInfo.file')} rows={[
        [t('titles:mediaInfo.container'), file.container?.toUpperCase() ?? ''],
        [t('titles:mediaInfo.size'), fmt.fileSize(file.size)],
        [t('titles:mediaInfo.overallBitrate'), rate(file.bitrate)],
        [t('titles:mediaInfo.duration'), file.duration_ms ? fmt.clock(file.duration_ms) : ''],
      ]} />
      <FilePath path={file.path} />
      {video && <Advanced video={video} />}
    </section>
  );
}

/** HDR as people name it; the source's formats, not what this device will output. */
function hdr(video: Stream, sdr: string, dvProfile: string): string {
  if (video.hdr_probe_status === 'unavailable' && !video.color_transfer) return '';
  const formats = fmt.sourceFormats(video).map(f => f === 'Dolby Vision' && video.dolby_vision_profile ? dvProfile : f);
  return formats.length ? formats.join(' · ') : sdr;
}

function Group({ label, children }: { label: string; children: ReactNode }) {
  return <div>
    <p className="mb-1.5 text-[11px] font-semibold uppercase tracking-wider text-subtle">{label}</p>
    <div className="overflow-hidden rounded-2xl bg-surface ring-1 ring-border">{children}</div>
  </div>;
}

/** Labelled values; a value the file doesn't have is left out rather than shown as a dash. */
function Fields({ label, rows }: { label: string; rows: [string, string][] }) {
  const shown = rows.filter(([, value]) => value);
  if (!shown.length) return null;
  return <Group label={label}>
    <dl className="divide-y divide-border text-sm">
      {shown.map(([name, value]) => <div key={name} className="grid grid-cols-[9rem_minmax(0,1fr)] gap-3 px-4 py-2">
        <dt className="text-muted">{name}</dt><dd className="min-w-0 break-words">{value}</dd>
      </div>)}
    </dl>
  </Group>;
}

/** One row per track; a column no track has a value for is left out. */
function Tracks({ label, columns, rows }: { label: string; columns: string[]; rows: string[][] }) {
  const keep = columns.map((_, i) => rows.some(r => r[i]));
  return <Group label={label}>
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="text-xs text-muted">
          <tr>{columns.map((c, i) => keep[i] && <th key={c} scope="col" className="px-4 pb-1.5 pt-2.5 font-medium">{c}</th>)}</tr>
        </thead>
        <tbody className="divide-y divide-border border-t border-border">
          {rows.map((r, n) => <tr key={n}>{r.map((v, i) => keep[i] && <td key={i} className="px-4 py-2 align-top">{v}</td>)}</tr>)}
        </tbody>
      </table>
    </div>
  </Group>;
}

function FilePath({ path }: { path: string }) {
  const { t } = useTranslation('titles');
  const [copied, setCopied] = useState(false);
  return <Group label={t('titles:mediaInfo.location')}>
    <div className="flex items-start gap-2 py-1.5 pl-4 pr-1.5">
      <p className="min-w-0 flex-1 break-all py-1 font-mono text-xs leading-relaxed text-muted">{path}</p>
      {navigator.clipboard && <Button size="sm" variant="ghost" className="shrink-0"
        onClick={() => navigator.clipboard.writeText(path).then(() => setCopied(true), () => {})}>
        {copied ? <Check className="size-4" /> : <Copy className="size-4" />} {copied ? t('titles:mediaInfo.copied') : t('titles:mediaInfo.copy')}
      </Button>}
    </div>
  </Group>;
}

/** The raw technical values, for anyone checking a file: closed until asked for. */
function Advanced({ video }: { video: Stream }) {
  const { t } = useTranslation('titles');
  const check = video.hdr_probe_status === 'sampled' ? t('titles:mediaInfo.checkSampled')
    : video.hdr_probe_status === 'stream' ? t('titles:mediaInfo.checkStream')
    : video.hdr_probe_status === 'unavailable' ? t('titles:mediaInfo.checkUnavailable') : '';
  const rows: [string, string][] = [
    [t('titles:mediaInfo.codecProfile'), video.profile ?? ''],
    [t('titles:mediaInfo.pixelFormat'), video.pix_fmt ?? ''],
    [t('titles:mediaInfo.colourPrimaries'), video.color_primaries ?? ''],
    [t('titles:mediaInfo.colourTransfer'), video.color_transfer ?? ''],
    [t('titles:mediaInfo.hdrCheck'), check],
  ];
  if (!rows.some(([, v]) => v)) return null;
  return <details className="group">
    <summary className="flex w-fit cursor-pointer list-none items-center gap-1 rounded-lg text-sm font-medium text-muted outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-accent [&::-webkit-details-marker]:hidden">
      <ChevronRight className="size-4 transition-transform group-open:rotate-90" />{t('titles:mediaInfo.advanced')}
    </summary>
    <div className="mt-2"><Fields label={t('titles:mediaInfo.video')} rows={rows} /></div>
  </details>;
}
