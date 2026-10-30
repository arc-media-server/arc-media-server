import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Play, SlidersHorizontal } from 'lucide-react';
import { api } from '@/lib/api';
import { usePrefs } from '@/lib/home';
import { audioLabel, chooseSubtitle, playerCaps, subtitleLabel, versionLabel } from '@/lib/playback';
import { usePlayer, type NowPlaying } from '@/components/Player';
import { ErrorNotice } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { Field, inputClass } from '@/components/ui/field';

/** A deliberate preplay path; the ordinary Play button keeps its automatic defaults. */
export function PlaybackOptionsButton({ item }: { item: NowPlaying }) {
  const { t } = useTranslation('titleActions');
  const [open, setOpen] = useState(false);
  return <>
    <Button variant="secondary" size="lg" className="size-12 rounded-2xl px-0" onClick={() => setOpen(true)}
      title={t("titleActions:playbackOptions.playbackOptions")} aria-label={t("titleActions:playbackOptions.playbackOptions")}>
      <SlidersHorizontal />
    </Button>
    <PlaybackOptionsDialog item={item} open={open} onOpenChange={setOpen} />
  </>;
}

/** Playback options as a dialog of its own, for opening from a "⋯" menu (an episode's). */
export function PlaybackOptionsDialog({ item, open, onOpenChange }: { item: NowPlaying; open: boolean; onOpenChange: (o: boolean) => void }) {
  const play = usePlayer();
  return <Dialog open={open} onOpenChange={onOpenChange}>
    {open && <Options key={`${item.itemId}:${item.edition ?? ''}`} item={item} onPlay={(selection, edition) => {
      onOpenChange(false);
      play({ ...item, selection, edition, subtitle: edition || (item.edition ? undefined : item.subtitle) });
    }} />}
  </Dialog>;
}

function Options({ item, onPlay }: { item: NowPlaying; onPlay: (selection: NonNullable<NowPlaying['selection']>, edition?: string) => void }) {
  const { t } = useTranslation('titleActions');
  const [fileId, setFileId] = useState<number>();
  const [audio, setAudio] = useState('');
  const [subtitle, setSubtitle] = useState('auto');
  const prefs = usePrefs();
  const options = useQuery({
    queryKey: ['playback-options', item.itemId, item.edition, fileId],
    queryFn: () => api.playbackOptions(item.itemId!, { caps: playerCaps(), file_id: fileId, edition: item.edition }),
    staleTime: 0,
  });
  const data = options.data;
  const selectedEdition = data?.versions.find(v => v.file_id === data.file_id)?.edition;
  const audioTrack = data?.audio_tracks.find(tabId => tabId.index === (audio === '' ? data.audio_stream : Number(audio)));
  const automaticSubtitle = data && prefs.data && chooseSubtitle(data.subtitle_tracks, prefs.data.subtitle_language, audioTrack?.language, prefs.data.subtitle_mode);
  const chosenSubtitle = data?.subtitle_tracks.find(tabId => tabId.id === subtitle);
  const label = (value: { name: string; detail: string }) => [value.name, value.detail].filter(Boolean).join(' · ');
  const ready = !!data && !!prefs.data && !options.isFetching && !options.error && !prefs.error;

  return <DialogContent title={t("titleActions:playbackOptions.playbackOptions")} description={[item.title, selectedEdition || (!item.edition ? item.subtitle : undefined)].filter(Boolean).join(' · ')} footer={<>
    <DialogClose asChild><Button variant="ghost">{t("titleActions:playbackOptions.cancel")}</Button></DialogClose>
    <Button variant="primary" disabled={!ready} onClick={() => data && onPlay({
      fileId: data.file_id,
      audioStream: audio === '' ? undefined : Number(audio),
      subtitle: subtitle === 'auto' ? undefined : subtitle === 'off' ? null : subtitle,
      burnSubtitle: chosenSubtitle?.image ? chosenSubtitle.id : undefined,
    }, selectedEdition)}><Play className="fill-current" />{item.startMS ? t("titleActions:playbackOptions.resume") : t("titleActions:playbackOptions.play")}</Button>
  </>}>
    <div className="space-y-5 pb-2">
      <ErrorNotice error={options.error} onRetry={() => void options.refetch()} busy={options.isFetching} />
      <ErrorNotice error={prefs.error} onRetry={() => void prefs.refetch()} busy={prefs.isFetching} />
      {(options.isPending || prefs.isPending) && <p role="status">{t("titleActions:playbackOptions.loadingPlaybackOptions")}</p>}
      {data && <>
        <Field label={t("titleActions:playbackOptions.version")}>
          <select className={inputClass} value={data.file_id} disabled={options.isFetching} onChange={e => {
            setFileId(Number(e.target.value));
            setAudio('');
            setSubtitle('auto');
          }}>
            {data.versions.map(v => <option key={v.file_id} value={v.file_id}>{label(versionLabel(v))}</option>)}
          </select>
        </Field>
        <Field label={t("titleActions:playbackOptions.audio")}>
          <select className={inputClass} value={audio} onChange={e => setAudio(e.target.value)} disabled={!ready || !data.audio_tracks.length}>
            <option value="">{t('titleActions:playbackOptions.automaticTrack', { track: data.audio_tracks.find(track => track.index === data.audio_stream) ? label(audioLabel(data.audio_tracks.find(track => track.index === data.audio_stream)!, 0)) : t('titleActions:playbackOptions.noAudio') })}</option>
            {data.audio_tracks.map((tabId, i) => <option key={tabId.index} value={tabId.index}>{label(audioLabel(tabId, i))}</option>)}
          </select>
        </Field>
        <Field label={t("titleActions:playbackOptions.subtitles")}>
          <select className={inputClass} value={subtitle} onChange={e => setSubtitle(e.target.value)} disabled={!ready}>
            <option value="auto">{t('titleActions:playbackOptions.automaticTrack', { track: automaticSubtitle ? label(subtitleLabel(automaticSubtitle, 0)) : t('titleActions:playbackOptions.off') })}</option>
            <option value="off">{t("titleActions:playbackOptions.off")}</option>
            {data.subtitle_tracks.map((tabId, i) => <option key={tabId.id} value={tabId.id} disabled={tabId.image && tabId.external}>
              {tabId.image && tabId.external ? t('titleActions:playbackOptions.unsupportedTrack', { track: label(subtitleLabel(tabId, i)) }) : label(subtitleLabel(tabId, i))}
            </option>)}
          </select>
        </Field>
        {chosenSubtitle?.image && <p className="text-sm text-muted">{t("titleActions:playbackOptions.pictureSubtitlesRequireVideo")}</p>}
        <p className="text-xs leading-relaxed text-subtle">{t("titleActions:playbackOptions.automaticUsesYourProfile")}</p>
        <p className="text-xs leading-relaxed text-subtle">{t("titleActions:playbackOptions.languageChoicesAreRemembered")}</p>
      </>}
    </div>
  </DialogContent>;
}
