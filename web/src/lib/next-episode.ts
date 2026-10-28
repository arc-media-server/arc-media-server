import type { SeasonEpisode } from './api';

/**
 * The episode the Play button starts: the one most recently watched if it's part-way through,
 * otherwise the next unwatched one after it, otherwise the first. Specials only if there's
 * nothing else.
 */
export function nextToPlay(all: SeasonEpisode[]): SeasonEpisode | undefined {
  const eps = all.filter((e) => e.available && e.stream_url);
  const main = eps.filter((e) => e.season_number > 0 || e.inline_special);
  const list = main.length ? main : eps;
  let last = -1;
  list.forEach((e, i) => {
    if ((e.user_data?.last_played_at ?? 0) > (list[last]?.user_data?.last_played_at ?? 0)) last = i;
  });
  if (last < 0) return list[0];
  if (list[last].user_data?.position_ms) return list[last];
  return list.slice(last + 1).find((e) => !e.user_data?.played) ?? list.find((e) => !e.user_data?.played) ?? list[0];
}
