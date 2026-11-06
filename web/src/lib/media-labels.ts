import i18n from '@/i18n';

/**
 * Display names for media formats in filters. Codec names come from ffprobe (lower case); format
 * and brand names aren't translated, but the words around them are.
 */
const codecNames: Record<string, string> = {
  hevc: 'HEVC (H.265)', h264: 'H.264', av1: 'AV1', vp9: 'VP9', vp8: 'VP8', mpeg2video: 'MPEG-2', mpeg4: 'MPEG-4',
  vc1: 'VC-1', wmv3: 'WMV', prores: 'ProRes',
  truehd: 'Dolby TrueHD', eac3: 'Dolby Digital Plus', ac3: 'Dolby Digital', dts: 'DTS', aac: 'AAC', flac: 'FLAC',
  opus: 'Opus', mp3: 'MP3', mp2: 'MP2', vorbis: 'Vorbis', alac: 'ALAC', wmav2: 'WMA',
};

export function codecLabel(codec: string): string {
  const c = codec.toLowerCase();
  if (codecNames[c]) return codecNames[c];
  if (c.startsWith('pcm_')) return 'PCM';
  return c.toUpperCase();
}

/** 6 → "5.1", 8 → "7.1", 2 → "Stereo". */
export function channelsLabel(channels: string | number): string {
  const n = Number(channels);
  if (n === 1) return i18n.t('filters:channels.mono');
  if (n === 2) return i18n.t('filters:channels.stereo');
  if (n === 3) return '2.1';
  if (n === 6) return '5.1';
  if (n === 7) return '6.1';
  if (n === 8) return '7.1';
  return i18n.t('filters:channels.count', { count: n });
}

const rangeNames: Record<string, string> = { dolby_vision: 'Dolby Vision', hdr10_plus: 'HDR10+', hdr10: 'HDR10', hlg: 'HLG', sdr: 'SDR' };

export function dynamicRangeLabel(range: string): string {
  return rangeNames[range] ?? i18n.t('filters:dynamicRange.unknown');
}
