import { api, type Item, type SeasonEpisode } from '@/lib/api';
import { nowPlayingFor, type NowPlaying } from '@/components/Player';

/** A copy of a list in a random order. */
export function shuffled<T>(list: T[]): T[] {
  const out = [...list];
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

/** Episodes on disk, ready for the player, each from its start. */
export function episodeQueue(episodes: SeasonEpisode[], seriesTitle: string): NowPlaying[] {
  return episodes.filter((e) => e.available && e.stream_url).map((e) => nowPlayingFor({ ...e, series_title: seriesTitle }, true));
}

/** A show's episodes on disk in its display order, leaving out specials (as Plex's Shuffle does). */
export async function showQueue(seriesId: string): Promise<NowPlaying[]> {
  const data = await api.seasons(seriesId);
  return episodeQueue(
    data.seasons.filter((s) => s.season_number !== 0).flatMap((s) => s.episodes),
    data.series.title,
  );
}

/** What a collection plays: its movies, and every episode of its shows. */
export async function collectionQueue(items: Item[]): Promise<NowPlaying[]> {
  const out: NowPlaying[] = [];
  for (const it of items) {
    if (it.kind === 'series') out.push(...(await showQueue(it.id)));
    else if (it.stream_url) out.push(nowPlayingFor(it, true));
  }
  return out;
}
