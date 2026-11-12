import type { ItemQuery } from './api';

const textKeys = ['genre', 'language', 'label', 'video_codec', 'audio_codec'] as const;
const numberKeys = ['decade', 'added_within', 'min_runtime', 'max_runtime', 'min_rating', 'audio_channels', 'person_id'] as const;
const flagKeys = ['unwatched', 'unmatched'] as const;
const enums = {
  resolution: ['4k', '1080', '720', 'sd'],
  dynamic_range: ['dolby_vision', 'hdr10_plus', 'hlg', 'hdr10', 'sdr', 'unknown'],
  watch_state: ['played', 'unstarted', 'unwatched'],
  person_role: ['actor', 'director'],
} as const;

/** Shared by Home row queries, library links and remembered library controls. */
export const browseFilterKeys = [...textKeys, ...numberKeys, ...flagKeys, ...Object.keys(enums) as (keyof typeof enums)[]];
export type ItemFilters = Pick<ItemQuery, typeof browseFilterKeys[number]>;

export function pickItemFilters(input: ItemFilters): ItemFilters {
  const filters: ItemFilters = {};
  for (const key of browseFilterKeys) {
    const value = input[key];
    if (value !== undefined) Object.assign(filters, { [key]: value });
  }
  // Keep the server alias usable with the existing Unwatched checkbox. Legacy links
  // containing both controls retain the same Unwatched precedence as the library.
  if (filters.unwatched || filters.watch_state === 'unwatched') {
    filters.unwatched = true;
    filters.watch_state = undefined;
  }
  return filters;
}

export function encodeItemFilters(input: ItemFilters): URLSearchParams {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(pickItemFilters(input))) {
    if (value !== undefined && value !== false && value !== '') params.set(key, value === true ? '1' : String(value));
  }
  return params;
}

/** Ignore corrupt/unknown URL or storage values rather than sending invalid API filters. */
export function readItemFilters(params: URLSearchParams): ItemFilters {
  const filters: ItemFilters = {};
  for (const key of textKeys) {
    const value = params.get(key);
    if (value && value.length <= 200) filters[key] = value;
  }
  for (const key of numberKeys) {
    const value = Number(params.get(key));
    if (Number.isFinite(value) && value > 0) filters[key] = value;
  }
  for (const key of flagKeys) if (params.get(key) === '1') filters[key] = true;
  for (const [key, values] of Object.entries(enums)) {
    const value = params.get(key);
    if (value && (values as readonly string[]).includes(value)) Object.assign(filters, { [key]: value });
  }
  if (!filters.person_id) delete filters.person_role;
  return pickItemFilters(filters);
}
