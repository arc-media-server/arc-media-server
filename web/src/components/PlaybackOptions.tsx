import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Play, SlidersHorizontal } from 'lucide-react';
import { api } from '@/lib/api';
import { usePrefs } from '@/lib/home';
import { audioLabel, chooseSubtitle, playerCaps, subtitleLabel, versionLabel } from '@/lib/playback';
import { usePlayer, type NowPlaying } from '@/components/Player';
import { ErrorNotice } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent, DialogTrigger } from '@/components/ui/dialog';
import { Field, inputClass } from '@/components/ui/field';

/** A deliberate preplay path; the ordinary Play button keeps its automatic defaults. */
export function PlaybackOptionsButton({ item, compact = false }: { item: NowPlaying; compact?: boolean }) {
  const [open, setOpen] = useState(false);
  const play = usePlayer();
  return <Dialog open={open} onOpenChange={setOpen}>
    <DialogTrigger asChild>
      <Button variant={compact ? 'ghost' : 'secondary'} size={compact ? 'icon' : 'lg'} className={compact ? undefined : 'size-12 rounded-2xl px-0'}
        title="Playback options" aria-label={compact ? `Playback options for ${item.title}` : 'Playback options'}>
        <SlidersHorizontal className={compact ? 'size-4' : undefined} />
      </Button>
    </DialogTrigger>
    {open && <Options key={`${item.itemId}:${item.edition ?? ''}`} item={item} onPlay={(selection, edition) => {
      setOpen(false);
      play({ ...item, selection, edition, subtitle: edition || (item.edition ? undefined : item.subtitle) });
    }} />}
  </Dialog>;
}

function Options({ item, onPlay }: { item: NowPlaying; onPlay: (selection: NonNullable<NowPlaying['selection']>, edition?: string) => void }) {
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
  const audioTrack = data?.audio_tracks.find(t => t.index === (audio === '' ? data.audio_stream : Number(audio)));
  const automaticSubtitle = data && prefs.data && chooseSubtitle(data.subtitle_tracks, prefs.data.subtitle_language, audioTrack?.language, prefs.data.subtitle_mode);
  const chosenSubtitle = data?.subtitle_tracks.find(t => t.id === subtitle);
  const label = (value: { name: string; detail: string }) => [value.name, value.detail].filter(Boolean).join(' · ');
  const ready = !!data && !!prefs.data && !options.isFetching && !options.error && !prefs.error;

  return <DialogContent title="Playback options" description={[item.title, selectedEdition || (!item.edition ? item.subtitle : undefined)].filter(Boolean).join(' · ')} footer={<>
    <DialogClose asChild><Button variant="ghost">Cancel</Button></DialogClose>
    <Button variant="primary" disabled={!ready} onClick={() => data && onPlay({
      fileId: data.file_id,
      audioStream: audio === '' ? undefined : Number(audio),
      subtitle: subtitle === 'auto' ? undefined : subtitle === 'off' ? null : subtitle,
      burnSubtitle: chosenSubtitle?.image ? chosenSubtitle.id : undefined,
    }, selectedEdition)}><Play className="fill-current" />{item.startMS ? 'Resume' : 'Play'}</Button>
  </>}>
    <div className="space-y-5 pb-2">
      <ErrorNotice error={options.error} onRetry={() => void options.refetch()} busy={options.isFetching} />
      <ErrorNotice error={prefs.error} onRetry={() => void prefs.refetch()} busy={prefs.isFetching} />
      {(options.isPending || prefs.isPending) && <p role="status">Loading playback options…</p>}
      {data && <>
        <Field label="Version">
          <select className={inputClass} value={data.file_id} disabled={options.isFetching} onChange={e => {
            setFileId(Number(e.target.value));
            setAudio('');
            setSubtitle('auto');
          }}>
            {data.versions.map(v => <option key={v.file_id} value={v.file_id}>{label(versionLabel(v))}</option>)}
          </select>
        </Field>
        <Field label="Audio">
          <select className={inputClass} value={audio} onChange={e => setAudio(e.target.value)} disabled={!ready || !data.audio_tracks.length}>
            <option value="">Automatic — {data.audio_tracks.find(t => t.index === data.audio_stream)
              ? label(audioLabel(data.audio_tracks.find(t => t.index === data.audio_stream)!, 0)) : 'No audio'}</option>
            {data.audio_tracks.map((t, i) => <option key={t.index} value={t.index}>{label(audioLabel(t, i))}</option>)}
          </select>
        </Field>
        <Field label="Subtitles">
          <select className={inputClass} value={subtitle} onChange={e => setSubtitle(e.target.value)} disabled={!ready}>
            <option value="auto">Automatic — {automaticSubtitle ? label(subtitleLabel(automaticSubtitle, 0)) : 'Off'}</option>
            <option value="off">Off</option>
            {data.subtitle_tracks.map((t, i) => <option key={t.id} value={t.id} disabled={t.image && t.external}>
              {label(subtitleLabel(t, i))}{t.image && t.external ? ' · Not supported in this browser' : ''}
            </option>)}
          </select>
        </Field>
        {chosenSubtitle?.image && <p className="text-sm text-muted">Picture subtitles require video conversion.</p>}
        <p className="text-xs leading-relaxed text-subtle">Automatic uses your profile preferences, falling back to this file’s available tracks. Choosing another version resets its track choices.</p>
        <p className="text-xs leading-relaxed text-subtle">Language choices are remembered when you play, just like in the player. Arc may adapt audio or video for this browser; source formats do not guarantee device output.</p>
      </>}
    </div>
  </DialogContent>;
}
