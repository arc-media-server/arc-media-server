import i18n from '@/i18n';
import type { SubtitleMode, AudioTrack, PlaybackPlan, PlayerCaps, SubtitleTrack, Version } from '@/lib/api';
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

const channelNames: Record<number, string> = { 6: '5.1', 8: '7.1' };
/** e.g. "English" with "Dolby Digital 5.1 · Commentary" as detail. */
export function audioLabel(t: AudioTrack, i: number) {
  const name = t.language && t.language !== 'und' ? fmt.language(t.language) : i18n.t('player:labels.track', { number: i + 1 });
  const detail = [fmt.codec(t.codec, t.profile), t.channels ? (t.channels <= 2 ? fmt.channels(t.channels) : channelNames[t.channels] ?? i18n.t('player:labels.channels', { count: t.channels })) : '', t.title]
    .filter(Boolean)
    .join(' · ');
  return { name, detail };
}

/** How the file is being sent, for the player's info menu. */
export function methodLabel(p: PlaybackPlan) {
  switch (p.method) {
    case 'direct':
      if (p.optimized) {
        const a = p.optimized.audio;
        const track = a ? audioLabel(a, 0) : undefined;
        return {
          title: i18n.t('player:method.optimizedTitle'),
          detail: track ? i18n.t('player:method.optimizedAudio', { quality: p.optimized.quality, name: track.name, detail: track.detail }) : i18n.t('player:method.optimizedSilent', { quality: p.optimized.quality }),
        };
      }
      return { title: i18n.t('player:method.directTitle'), detail: i18n.t('player:method.directDescription') };
    case 'remux':
      return { title: i18n.t('player:method.remuxTitle'), detail: i18n.t('player:method.remuxDescription', { reasons: p.reasons.join(', ') }) };
    case 'transcode':
      return {
        title: i18n.t('player:method.convertingTitle'),
        detail: p.encoder ? p.tone_mapped ? i18n.t('player:method.hardwareHdr', { reasons: p.reasons.join(', '), encoder: p.encoder }) : i18n.t('player:method.hardware', { reasons: p.reasons.join(', '), encoder: p.encoder }) : p.tone_mapped ? i18n.t('player:method.softwareHdr', { reasons: p.reasons.join(', '), encoder: p.encoder }) : i18n.t('player:method.software', { reasons: p.reasons.join(', '), encoder: p.encoder }),
      };
  }
}

/** e.g. "English" with "Forced · SDH · File" as detail. */
export function subtitleLabel(t: SubtitleTrack, i: number) {
  const name = t.language && t.language !== 'und' ? fmt.language(t.language) : t.title || i18n.t('player:labels.subtitlesNumber', { number: i + 1 });
  const detail = [
    t.forced && i18n.t('player:labels.forced'),
    t.hearing_impaired && 'SDH',
    t.title && t.title !== name && !/^(forced|sdh)$/i.test(t.title) ? t.title : '',
    t.image ? i18n.t('player:labels.picture') : '',
    t.downloaded ? i18n.t('player:labels.downloaded') : t.external ? i18n.t('player:labels.file') : '',
  ]
    .filter(Boolean)
    .join(' · ');
  return { name, detail };
}

/**
 * The subtitles to start with: the profile's chosen language (full subtitles rather than forced
 * ones), or else forced subtitles for the audio's language (signs and foreign dialogue).
 */
export function chooseSubtitle(tracks: SubtitleTrack[], preferred: string, audioLanguage?: string, mode: SubtitleMode = ''): SubtitleTrack | undefined {
  const same = (a?: string, b?: string) => !!a && !!b && fmt.language(a) === fmt.language(b);
  // Plex's subtitle modes: only forced ones by themselves, or the profile's language when the
  // audio is in another one, or always; "" turns on the language last chosen.
  const wanted = mode === 'manual' ? false : mode === 'foreign' ? !!preferred && !!audioLanguage && !same(audioLanguage, preferred) : true;
  if (preferred && wanted) {
    const matching = tracks.filter((t) => same(t.language, preferred));
    const best = matching.find((t) => !t.forced && !t.hearing_impaired && !t.image) ?? matching.find((t) => !t.forced) ?? matching[0];
    if (best) return best;
  }
  return tracks.find((t) => t.forced && (!t.language || !audioLanguage || same(t.language, audioLanguage)));
}

/** The quality limits the player offers (kbit/s), as Plex's: each converts to a size that suits it. */
export const qualities = [20000, 12000, 8000, 4000, 3000, 2000, 1500, 720];

/** e.g. "4 Mbps 720p" or "720 kbps 360p". */
export function qualityName(kbps: number) {
  const height = kbps >= 8000 ? 1080 : kbps >= 2000 ? 720 : kbps >= 1500 ? 480 : 360;
  return `${kbps >= 1000 ? `${kbps / 1000} Mbps` : `${kbps} kbps`} ${height}p`;
}

/** Device choice: -1 Automatic, 0 explicit Original, or a saved manual limit. */
export function readQuality(): number {
  try {
    const saved = localStorage.getItem('arc-quality');
    if (saved === null || saved === 'auto') return -1;
    const v = Number(saved);
    return v === 0 || qualities.includes(v) ? v : -1;
  } catch {
    return -1;
  }
}

export function saveQuality(kbps: number) {
  try {
    localStorage.setItem('arc-quality', kbps === -1 ? 'auto' : String(kbps));
  } catch {
    // Private browsing: the choice lasts for this playback only.
  }
}

/** e.g. "Director's Cut" with "4K · HDR10 · HEVC · 58 GB", or "4K" with "HDR10 · HEVC · 58 GB". */
export function versionLabel(v: Version) {
  const res = v.width && v.height ? fmt.resolution(v.width, v.height) : '';
  const rest = [...(v.source_formats ?? (v.hdr ? [v.hdr] : [])), v.video_codec ? fmt.codec(v.video_codec) : '', fmt.fileSize(v.size)];
  return v.edition ? { name: v.edition, detail: [res, ...rest].filter(Boolean).join(' · ') } : { name: res || i18n.t('player:labels.version'), detail: rest.filter(Boolean).join(' · ') };
}

/**
 * Devices to check a library against. Browsers' support is what they can do on most computers;
 * HEVC and Dolby audio depend on the hardware and system, so they're left out where uncertain.
 */
export const devicePresets: { id: string; label: string; caps: () => PlayerCaps }[] = [
  { id: 'this', get label() { return i18n.t('player:labels.thisBrowser'); }, caps: playerCaps },
  {
    id: 'chrome',
    label: 'Chrome',
    caps: () => ({ containers: ['mp4', 'webm'], video: ['h264', 'vp9', 'av1'], audio: ['aac', 'mp3', 'opus', 'flac', 'vorbis'], max_height: 2160 }),
  },
  {
    id: 'edge',
    get label() { return i18n.t('player:labels.edgeWindows'); },
    caps: () => ({ containers: ['mp4', 'webm'], video: ['h264', 'hevc', 'vp9', 'av1'], audio: ['aac', 'mp3', 'opus', 'flac', 'vorbis'], max_height: 2160 }),
  },
  {
    id: 'firefox',
    label: 'Firefox',
    caps: () => ({ containers: ['mp4', 'webm'], video: ['h264', 'vp9', 'av1'], audio: ['aac', 'mp3', 'opus', 'flac', 'vorbis'], max_height: 2160 }),
  },
  {
    id: 'safari',
    get label() { return i18n.t('player:labels.safariMac'); },
    caps: () => ({ containers: ['mp4'], video: ['h264', 'hevc', 'vp9'], audio: ['aac', 'mp3', 'ac3', 'eac3', 'flac', 'opus'], max_height: 2160 }),
  },
  {
    id: 'ios',
    get label() { return i18n.t('player:labels.iphoneIpad'); },
    caps: () => ({ containers: ['mp4'], video: ['h264', 'hevc'], audio: ['aac', 'mp3', 'ac3', 'eac3', 'flac'], max_height: 2160 }),
  },
];

/** The existing player learning policy: forced subtitles do not replace the preferred language. */
export function rememberedSubtitleLanguage(track: SubtitleTrack | null, mode: SubtitleMode = ''): string | undefined {
  if (track && !track.forced) return track.language ?? '';
  if (!track && !mode) return '';
  return undefined;
}
