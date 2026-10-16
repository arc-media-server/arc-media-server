import type { MediaFile, Stream } from './api';

export function runtime(minutes?: number): string {
  if (!minutes) return '';
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return `${m}m`;
  return m === 0 ? `${h}h` : `${h}h ${m}m`;
}

export function durationMinutes(ms?: number): number {
  return ms ? Math.round(ms / 60000) : 0;
}

export function resolution(width?: number, height?: number): string {
  if (!width || !height) return '';
  if (width >= 3200 || height >= 1600) return '4K';
  if (width >= 1800 || height >= 1000) return '1080p';
  if (width >= 1200 || height >= 700) return '720p';
  return 'SD';
}

const codecNames: Record<string, string> = {
  h264: 'H.264',
  hevc: 'HEVC',
  h265: 'HEVC',
  av1: 'AV1',
  vp9: 'VP9',
  vp8: 'VP8',
  mpeg2video: 'MPEG-2',
  mpeg4: 'MPEG-4',
  vc1: 'VC-1',
  aac: 'AAC',
  ac3: 'Dolby Digital',
  eac3: 'Dolby Digital+',
  truehd: 'Dolby TrueHD',
  dts: 'DTS',
  flac: 'FLAC',
  opus: 'Opus',
  mp3: 'MP3',
  vorbis: 'Vorbis',
  pcm_s16le: 'PCM',
  pcm_s24le: 'PCM',
  subrip: 'SRT',
  ass: 'ASS',
  ssa: 'SSA',
  mov_text: 'Text',
  hdmv_pgs_subtitle: 'PGS',
  dvd_subtitle: 'VobSub',
  webvtt: 'WebVTT',
};

export function codec(name?: string): string {
  if (!name) return '';
  return codecNames[name] ?? name.toUpperCase();
}

export function channels(n?: number): string {
  switch (n) {
    case undefined:
    case 0:
      return '';
    case 1:
      return 'Mono';
    case 2:
      return 'Stereo';
    case 6:
      return '5.1';
    case 8:
      return '7.1';
    default:
      return `${n}ch`;
  }
}

const languageNames = new Intl.DisplayNames(['en'], { type: 'language' });

export function language(code?: string): string {
  if (!code || code === 'und') return '';
  try {
    return languageNames.of(code) ?? code;
  } catch {
    return code;
  }
}

export function fileSize(bytes: number): string {
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`;
  if (bytes >= 1e6) return `${Math.round(bytes / 1e6)} MB`;
  return `${Math.round(bytes / 1e3)} KB`;
}

export function date(iso?: string): string {
  if (!iso) return '';
  const d = new Date(`${iso}T00:00:00`);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
}

/** "Today, 23:40", "Yesterday, 08:15" or "12 Oct 2026, 03:00". */
export function dateTime(iso?: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const time = d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
  const days = Math.round((startOfDay(new Date()) - startOfDay(d)) / 86_400_000);
  if (days === 0) return `Today, ${time}`;
  if (days === 1) return `Yesterday, ${time}`;
  return `${d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })}, ${time}`;
}

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

export function videoStream(file?: MediaFile): Stream | undefined {
  return file?.streams.find((s) => s.kind === 'video');
}

export function audioStreams(file?: MediaFile): Stream[] {
  return file?.streams.filter((s) => s.kind === 'audio') ?? [];
}

export function subtitleStreams(file?: MediaFile): Stream[] {
  return file?.streams.filter((s) => s.kind === 'subtitle') ?? [];
}

export function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? '' : 's'}`;
}
