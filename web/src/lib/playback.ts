import type { AudioTrack, PlaybackPlan, PlayerCaps, SubtitleTrack, Version } from '@/lib/api';
import * as fmt from '@/lib/format';

let cached: PlayerCaps | undefined;

/** What this browser can play, so the server can send files as they are when it can. */
export function playerCaps(): PlayerCaps {
  if (cached) return cached;
  const v = document.createElement('video');
  const mse = (type: string) => (typeof MediaSource !== 'undefined' && MediaSource.isTypeSupported(type)) || v.canPlayType(type) === 'probably';
  const pick = (tests: Record<string, string[]>) => Object.keys(tests).filter((k) => tests[k].some(mse));
  cached = {
    // Matroska isn't listed: browsers open some MKV files but not reliably, so they're repackaged.
    containers: ['mp4', ...(v.canPlayType('video/webm') ? ['webm'] : [])],
    video: pick({
      h264: ['video/mp4; codecs="avc1.640028"'],
      hevc: ['video/mp4; codecs="hvc1.1.6.L120.90"', 'video/mp4; codecs="hev1.1.6.L120.90"'],
      vp9: ['video/webm; codecs="vp9"', 'video/mp4; codecs="vp09.00.10.08"'],
      av1: ['video/mp4; codecs="av01.0.08M.08"'],
    }),
    audio: pick({
      aac: ['audio/mp4; codecs="mp4a.40.2"'],
      mp3: ['audio/mpeg', 'audio/mp4; codecs="mp3"'],
      opus: ['audio/webm; codecs="opus"', 'audio/mp4; codecs="opus"'],
      flac: ['audio/mp4; codecs="flac"', 'audio/flac'],
      vorbis: ['audio/webm; codecs="vorbis"'],
      ac3: ['audio/mp4; codecs="ac-3"'],
      eac3: ['audio/mp4; codecs="ec-3"'],
    }),
    max_height: Math.round(window.screen.height * (window.devicePixelRatio || 1)),
  };
  return cached;
}

/**
 * Caps for retrying after a method failed (browsers sometimes claim more than they can do): the
 * file as it is failed, so repackage it; repackaged failed, so convert the video too.
 */
export function fallbackCaps(failed: PlaybackPlan['method']): PlayerCaps {
  const caps = playerCaps();
  return failed === 'direct' ? { ...caps, containers: [] } : { ...caps, containers: [], video: [] };
}

const channelNames: Record<number, string> = { 1: 'Mono', 2: 'Stereo', 6: '5.1', 8: '7.1' };
const audioCodecs: Record<string, string> = {
  aac: 'AAC',
  ac3: 'Dolby Digital',
  eac3: 'Dolby Digital+',
  truehd: 'TrueHD',
  dts: 'DTS',
  flac: 'FLAC',
  mp3: 'MP3',
  opus: 'Opus',
  vorbis: 'Vorbis',
};

/** e.g. "English" with "Dolby Digital 5.1 · Commentary" as detail. */
export function audioLabel(t: AudioTrack, i: number) {
  const name = t.language && t.language !== 'und' ? fmt.language(t.language) : `Track ${i + 1}`;
  const detail = [audioCodecs[t.codec] ?? t.codec.toUpperCase(), t.channels ? (channelNames[t.channels] ?? `${t.channels} ch`) : '', t.title]
    .filter(Boolean)
    .join(' · ');
  return { name, detail };
}

/** How the file is being sent, for the player's info menu. */
export function methodLabel(p: PlaybackPlan) {
  switch (p.method) {
    case 'direct':
      return { title: 'Direct Play', detail: 'The original file, exactly as it is.' };
    case 'remux':
      return { title: 'Repackaged', detail: `The video is sent as it is; Arc adapts the rest for this browser (${p.reasons.join(', ')}).` };
    case 'transcode':
      return {
        title: 'Converting',
        detail:
          `Arc is converting the video for this browser (${p.reasons.join(', ')})` +
          (p.encoder ? ` with the graphics card (${p.encoder})` : ' with the processor') +
          (p.tone_mapped ? ', mapping HDR to SDR.' : '.'),
      };
  }
}

/** e.g. "English" with "Forced · SDH · File" as detail. */
export function subtitleLabel(t: SubtitleTrack, i: number) {
  const name = t.language && t.language !== 'und' ? fmt.language(t.language) : t.title || `Subtitles ${i + 1}`;
  const detail = [
    t.forced && 'Forced',
    t.hearing_impaired && 'SDH',
    t.title && t.title !== name && !/^(forced|sdh)$/i.test(t.title) ? t.title : '',
    t.image ? 'Picture' : '',
    t.external ? 'File' : '',
  ]
    .filter(Boolean)
    .join(' · ');
  return { name, detail };
}

/**
 * The subtitles to start with: the profile's chosen language (full subtitles rather than forced
 * ones), or else forced subtitles for the audio's language (signs and foreign dialogue).
 */
export function chooseSubtitle(tracks: SubtitleTrack[], preferred: string, audioLanguage?: string): SubtitleTrack | undefined {
  const same = (a?: string, b?: string) => !!a && !!b && fmt.language(a) === fmt.language(b);
  if (preferred) {
    const matching = tracks.filter((t) => same(t.language, preferred));
    const best = matching.find((t) => !t.forced && !t.hearing_impaired && !t.image) ?? matching.find((t) => !t.forced) ?? matching[0];
    if (best) return best;
  }
  return tracks.find((t) => t.forced && (!t.language || !audioLanguage || same(t.language, audioLanguage)));
}

/** e.g. "Director's Cut" with "4K · HDR10 · HEVC · 58 GB", or "4K" with "HDR10 · HEVC · 58 GB". */
export function versionLabel(v: Version) {
  const res = v.width && v.height ? fmt.resolution(v.width, v.height) : '';
  const rest = [v.hdr, v.video_codec ? fmt.codec(v.video_codec) : '', fmt.fileSize(v.size)];
  return v.edition ? { name: v.edition, detail: [res, ...rest].filter(Boolean).join(' · ') } : { name: res || 'Version', detail: rest.filter(Boolean).join(' · ') };
}
