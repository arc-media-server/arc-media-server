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
    <DialogTrigger asChild><Button size="icon" variant="ghost" aria-label={t('titles:sourceFormats.forTitle', { title })}><Info className="size-4" /></Button></DialogTrigger>
    <DialogContent title={t('titles:sourceFormats.title', { title })} description={t("titles:sourceFormats.description")}>
      <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />
      {query.isPending && <p role="status">{t("titles:sourceFormats.loadingFileDetails")}</p>}
      {query.data?.files?.map(f => <FileDetails key={f.id} file={f} several={(query.data?.files?.length ?? 0) > 1} compact />)}
    </DialogContent>
  </Dialog>;
}

export function FileDetails({ file, several, compact }: { file: MediaFile; several?: boolean; compact?: boolean }) {
  const { t } = useTranslation('titles');
  const video = fmt.videoStream(file);
  const audio = fmt.audioStreams(file);
  const subs = fmt.subtitleStreams(file);
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
          {video && <span className="block text-subtle">{[`${video.width}×${video.height}`, fmt.aspectRatio(video.aspect_ratio)].filter(Boolean).join(' · ')}</span>}
        </Detail>
        <Detail label={t("titles:sourceFormats.audio")}>
          {audio.length === 0
            ? '—'
            : audio.map((a) => (
                <span key={a.index} className="block">
                  {[fmt.language(a.language), fmt.codec(a.codec, a.profile), fmt.channels(a.channels)].filter(Boolean).join(' · ')}
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
          {[file.container?.toUpperCase(), fmt.fileSize(file.size)].filter(Boolean).join(' · ')}
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
