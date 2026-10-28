import i18n from '../i18n';
import type { MediaFile, Stream } from './api';

export function runtime(minutes?: number): string {
  if (!minutes) return '';
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return i18n.t('common:format.minutes', { count: m });
  return m === 0 ? i18n.t('common:format.hours', { count: h }) : i18n.t('common:time.hoursMinutes', { hours: h, minutes: m });
}

/** A position like "1:04:09" or "12:30". */
export function clock(ms: number): string {
  const total = Math.floor(ms / 1000);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = String(total % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`;
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

/** A display aspect ratio as people say it: 4:3 and 16:9 by name, wider films as e.g. 2.39:1. */
export function aspectRatio(ratio?: number): string {
  if (!ratio) return '';
  if (Math.abs(ratio - 4 / 3) < 0.02) return '4:3';
  if (Math.abs(ratio - 16 / 9) < 0.02) return '16:9';
  return `${ratio.toFixed(2)}:1`;
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

export function codec(name?: string, profile?: string): string {
  if (!name) return '';
  const p = profile?.toLowerCase() ?? '';
  if (name === 'truehd' && p === 'dolby truehd + dolby atmos') return 'TrueHD Atmos';
  if (name === 'eac3' && p === 'dolby digital plus + dolby atmos') return 'Dolby Digital+ Atmos';
  if (name === 'dts') {
    if (p === 'dts-hd ma + dts:x' || p === 'dts-hd ma + dts:x imax') return 'DTS-HD MA · DTS:X';
    if (p === 'dts-hd ma') return 'DTS-HD MA';
    if (p === 'dts-hd hra') return 'DTS-HD HRA';
    if (p === 'dts express') return 'DTS Express';
  }
  return codecNames[name] ?? name.toUpperCase();
}

export function channels(n?: number): string {
  switch (n) {
    case undefined:
    case 0:
      return '';
    case 1:
      return i18n.t('common:format.mono');
    case 2:
      return i18n.t('common:format.stereo');
    case 6:
      return '5.1';
    case 8:
      return '7.1';
    default:
      return i18n.t('common:format.channels', { count: n });
  }
}

// Keep the existing British English dates/numbers even with an en-US browser.
export function formatLocale(): string {
  const active = i18n.resolvedLanguage ?? 'en';
  return active === 'en' || active.startsWith('en-') ? 'en-GB' : active;
}

export function number(value: number, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(formatLocale(), options).format(value);
}

export function fixed(value: number, digits: number): string {
  // toFixed's binary rounding is part of the existing English file-size display.
  return formatLocale() === 'en-GB' ? value.toFixed(digits) : number(value, { useGrouping: false, minimumFractionDigits: digits, maximumFractionDigits: digits });
}

export function shortDate(iso: string): string {
  return new Date(iso).toLocaleDateString(formatLocale(), { day: 'numeric', month: 'short' });
}

const languageNames = new Map<string, Intl.DisplayNames>();

export function language(code?: string): string {
  if (!code || code === 'und') return '';
  try {
    const locale = formatLocale();
    let names = languageNames.get(locale);
    if (!names) {
      names = new Intl.DisplayNames([locale], { type: 'language' });
      languageNames.set(locale, names);
    }
    return names.of(code) ?? code;
  } catch {
    return code;
  }
}

export function fileSize(bytes: number): string {
  if (bytes >= 1e12) return i18n.t('common:format.terabytes', { value: fixed(bytes / 1e12, 1) });
  if (bytes >= 1e9) return i18n.t('common:format.gigabytes', { value: fixed(bytes / 1e9, 1) });
  if (bytes >= 1e6) return i18n.t('common:format.megabytes', { value: number(Math.round(bytes / 1e6), { useGrouping: false }) });
  return i18n.t('common:format.kilobytes', { value: number(Math.round(bytes / 1e3), { useGrouping: false }) });
}

export function date(iso?: string): string {
  if (!iso) return '';
  const d = new Date(`${iso}T00:00:00`);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(formatLocale(), { day: 'numeric', month: 'short', year: 'numeric' });
}

/** "Today, 23:40", "Yesterday, 08:15" or "12 Oct 2026, 03:00". */
export function dateTime(iso?: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const time = d.toLocaleTimeString(formatLocale(), { hour: '2-digit', minute: '2-digit' });
  const days = Math.round((startOfDay(new Date()) - startOfDay(d)) / 86_400_000);
  if (days === 0) return i18n.t('common:time.todayAt', { time });
  if (days === 1) return i18n.t('common:time.yesterdayAt', { time });
  return i18n.t('common:time.dateAt', { date: d.toLocaleDateString(formatLocale(), { day: 'numeric', month: 'short', year: 'numeric' }), time });
}

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
}

export function videoStream(file?: MediaFile): Stream | undefined {
  return file?.streams.find((s) => s.kind === 'video');
}

/** Labels describe source metadata, not HDR/Atmos output on the current device. */
export function sourceFormats(video?: Stream): string[] {
  if (!video) return [];
  const labels: string[] = [];
  if (video.dolby_vision) labels.push('Dolby Vision');
  if (video.hdr10_plus) labels.push('HDR10+');
  if (!labels.length && video.color_transfer === 'smpte2084') labels.push('HDR10');
  if (video.color_transfer === 'arib-std-b67') labels.push('HLG');
  return labels;
}

export function audioFormat(audio?: Stream): string {
  return audio ? [codec(audio.codec, audio.profile), channels(audio.channels)].filter(Boolean).join(' ') : '';
}

export function audioStreams(file?: MediaFile): Stream[] {
  return file?.streams.filter((s) => s.kind === 'audio') ?? [];
}

export function subtitleStreams(file?: MediaFile): Stream[] {
  return file?.streams.filter((s) => s.kind === 'subtitle') ?? [];
}

/** What a running scan is doing, e.g. "Matching movies · 120 of 1,812 · 98 identified". */
export function scanProgress(scan: { files_seen: number; matched: number; phase?: string; done?: number; total?: number }): string {
  const parts = [
    scan.phase ? scan.phase : i18n.t('common:activity.lookingThroughFolders', { count: scan.files_seen }),
    scan.phase && scan.total ? i18n.t('common:activity.progress', { done: number(Math.min(scan.done ?? 0, scan.total)), total: number(scan.total) }) : '',
    scan.matched ? i18n.t('common:activity.identified', { count: scan.matched, value: number(scan.matched) }) : '',
  ];
  return parts.filter(Boolean).join(' · ');
}

export function plural(n: number, word: string, pluralWord = `${word}s`): string {
  // Compatibility for pages converted in later phases; new text uses noun-specific keys.
  return i18n.t('common:format.legacyPlural', { count: n, word, pluralWord });
}

/** Time spent watching: "45m", "3h 20m", or "128 hours" for long stretches. */
export function watchTime(ms: number): string {
  const minutes = Math.round(ms / 60_000);
  if (minutes < 60) return i18n.t('common:format.minutes', { count: minutes });
  const hours = Math.floor(minutes / 60);
  if (hours >= 100) return i18n.t('common:format.longHours', { count: hours, value: number(hours) });
  return minutes % 60 ? i18n.t('common:time.hoursMinutes', { hours, minutes: minutes % 60 }) : i18n.t('common:format.hours', { count: hours });
}

/** How long ago a unix time was: "today", "3 days ago", "2 months ago", "over a year ago". */
export function ago(unix: number): string {
  const days = Math.floor((Date.now() / 1000 - unix) / 86_400);
  if (days < 1) return i18n.t('common:time.today');
  if (days === 1) return i18n.t('common:time.yesterday');
  if (days < 30) return i18n.t('common:time.daysAgo', { count: days });
  const months = Math.floor(days / 30);
  if (months < 12) return i18n.t('common:time.monthsAgo', { count: months });
  const years = Math.floor(days / 365);
  return i18n.t('common:time.yearsAgo', { count: Math.max(1, years) });
}

/** Calendar month (1–12) in the active UI language. */
export function month(value: number, style: 'short' | 'long' = 'short'): string {
  return new Intl.DateTimeFormat(formatLocale(), { month: style }).format(new Date(2020, value - 1, 1));
}

/** Calendar weekday label, anchored to a known Sunday independently of the machine timezone. */
export function weekday(value: number): string {
  return new Intl.DateTimeFormat(formatLocale(), { weekday: 'short', timeZone: 'UTC' }).format(new Date(Date.UTC(2020, 5, 7 + value)));
}
