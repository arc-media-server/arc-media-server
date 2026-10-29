import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Info } from 'lucide-react';
import { api, type MediaFile } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogTrigger } from '@/components/ui/dialog';
import { ErrorNotice } from '@/components/Page';

/** Fetch episode file metadata only on demand; never infer a format for a whole series. */
export function SourceFilesButton({ id, title }: { id: string; title: string }) {
  const { t } = useTranslation('titles');
  const [open, setOpen] = useState(false);
  const query = useQuery({ queryKey: ['item', id], queryFn: () => api.item(id), enabled: open });
  return <Dialog open={open} onOpenChange={setOpen}>
    <DialogTrigger asChild><Button size="sm" variant="ghost" aria-label={t('titles:sourceFormats.forTitle', { title })}><Info className="size-4" />{t("titles:sourceFormats.mediaInformation")}</Button></DialogTrigger>
    <DialogContent title={t('titles:sourceFormats.title', { title })} description={t("titles:sourceFormats.description")}>
      <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />
      {query.isPending && <p role="status">{t("titles:sourceFormats.loadingFileDetails")}</p>}
      {query.data && !query.data.files?.length && <p className="text-sm text-muted">{t("titles:sourceFormats.noFiles")}</p>}
      {query.data?.files?.map(f => <FileDetails key={f.id} file={f} several={(query.data?.files?.length ?? 0) > 1} compact />)}
    </DialogContent>
  </Dialog>;
}

export function FileDetails({ file, several, compact }: { file: MediaFile; several?: boolean; compact?: boolean }) {
  const { t } = useTranslation('titles');
  const video = fmt.videoStream(file);
  const audio = fmt.audioStreams(file);
  const subs = fmt.subtitleStreams(file);
  const rate = (value?: number) => value && value > 0 ? value >= 1e6
    ? t('titles:sourceFormats.megabits', { value: fmt.fixed(value / 1e6, 1) })
    : t('titles:sourceFormats.kilobits', { value: fmt.fixed(value / 1e3, 0) }) : '';
  const inspection = video?.hdr_probe_status === 'sampled' ? t('titles:sourceFormats.sampled')
    : video?.hdr_probe_status === 'stream' ? t('titles:sourceFormats.streamInspection')
    : video?.hdr_probe_status === 'unavailable' ? t('titles:sourceFormats.inspectionUnavailable') : '';

  return (
    <section className={compact ? "mb-6" : "mx-6 mt-10 md:mx-10"}>
      <h2 className="mb-4 text-lg font-semibold">
        {several
          ? t('titles:sourceFormats.version', { details: [file.edition, video && fmt.resolution(video.width, video.height), file.part > 0 && t('titles:sourceFormats.part', { number: file.part })].filter(Boolean).map(value => ` ${value}`).join('') })
          : t("titles:sourceFormats.aboutThisFile")}
      </h2>
      <div className={`grid gap-px overflow-hidden rounded-2xl bg-border ring-1 ring-border sm:grid-cols-2 ${compact ? "" : "lg:grid-cols-4"}`}>
        <Detail label={t("titles:sourceFormats.video")}>
          {video ? [fmt.resolution(video.width, video.height), fmt.codec(video.codec), ...fmt.sourceFormats(video)].filter(Boolean).join(' · ') : '—'}
          {video && <span className="block text-subtle">{[video.width && video.height ? `${video.width}×${video.height}` : undefined, fmt.aspectRatio(video.aspect_ratio)].filter(Boolean).join(' · ')}</span>}
          {!!video?.frame_rate && <span className="block text-subtle">{t('titles:sourceFormats.frameRate', { value: fmt.fixed(video.frame_rate, 3) })}</span>}
          {!!video?.bitrate && <span className="block text-subtle">{t('titles:sourceFormats.trackBitrate', { value: rate(video.bitrate) })}</span>}
          {!!video?.bit_depth && <span className="block text-subtle">{t('titles:sourceFormats.bitDepth', { count: video.bit_depth })}</span>}
          {!!video?.dolby_vision_profile && <span className="block text-subtle">{t('titles:sourceFormats.dvProfile', { profile: video.dolby_vision_profile })}</span>}
          {(video?.color_primaries || video?.color_transfer || video?.pix_fmt) && <span className="block text-subtle">{[video.color_primaries, video.color_transfer, video.pix_fmt].filter(Boolean).join(' · ')}</span>}
          {inspection && <span className="block text-subtle">{inspection}</span>}
        </Detail>
        <Detail label={t("titles:sourceFormats.audio")}>
          {audio.length === 0
            ? '—'
            : audio.map((a) => (
                <span key={a.index} className="block">
                  {[fmt.language(a.language), fmt.codec(a.codec, a.profile), fmt.channels(a.channels), rate(a.bitrate)].filter(Boolean).join(' · ')}
                </span>
              ))}
        </Detail>
        <Detail label={t("titles:sourceFormats.subtitles")}>
          {subs.length === 0
            ? t("titles:sourceFormats.none")
            : subs.map((s) => (
                <span key={s.index} className="block">
                  {[fmt.language(s.language) || t('titles:sourceFormats.unknown'), fmt.codec(s.codec), s.is_forced && t('titles:sourceFormats.forced')].filter(Boolean).join(' · ')}
                </span>
              ))}
        </Detail>
        <Detail label={t("titles:sourceFormats.file")}>
          {[file.container?.toUpperCase(), fmt.fileSize(file.size), rate(file.bitrate)].filter(Boolean).join(' · ')}
          {file.missing && <span className="block text-warning">{t("titles:sourceFormats.missingFromDisk")}</span>}
        </Detail>
      </div>
      <p className="mt-3 text-xs text-subtle">{t("titles:sourceFormats.sourceDisclaimer")}</p>
      <p className="mt-3 break-all font-mono text-xs text-subtle">{file.path}</p>
      {file.probe_error && <p className="mt-2 text-xs text-warning">{t('titles:sourceFormats.probeError', { error: file.probe_error })}</p>}
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
