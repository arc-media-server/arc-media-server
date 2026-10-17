import { Link } from 'react-router';
import { Play } from 'lucide-react';
import type { Item } from '@/lib/api';
import { cn } from '@/lib/utils';
import { useSpoilerGuard } from '@/lib/home';
import { Artwork } from './Artwork';
import { usePlayer } from './Player';
import { episodeCode, ProgressBar, progressOf } from './WatchState';

/**
 * A landscape card for something to carry on with (Continue Watching, Up Next): clicking the
 * artwork plays it from where the profile left off.
 */
export function ResumeCard({ item, className }: { item: Item; className?: string }) {
  const play = usePlayer();
  const isEpisode = item.kind === 'episode';
  const code = episodeCode(item.season_number, item.episode_number);
  const title = isEpisode ? (item.series_title ?? item.title) : item.title;
  const subtitle = isEpisode ? [code, item.title].filter(Boolean).join(' · ') : item.year ? String(item.year) : '';
  // With spoiler protection, an episode not started yet shows its show's artwork instead of its still.
  const hideSpoilers = useSpoilerGuard()(item);
  const art = (hideSpoilers && item.images?.series_backdrop) || (item.images?.primary ?? item.images?.series_backdrop ?? item.images?.backdrop);
  const blur = hideSpoilers && art === item.images?.primary;
  const href = isEpisode ? `/series/${item.parent_id}` : `/movie/${item.id}`;
  const left = remaining(item);

  return (
    <div className={cn('group', className)}>
      <button
        type="button"
        onClick={() =>
          item.stream_url &&
          play({ title: isEpisode ? item.title : item.title, subtitle: isEpisode ? `${title} · ${code}` : subtitle, src: item.stream_url, itemId: item.id, startMS: item.user_data?.position_ms })
        }
        className="relative block w-full overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border outline-none transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl focus-visible:ring-2 focus-visible:ring-accent"
        aria-label={`Play ${title}`}
      >
        <Artwork src={art} alt="" kind="still" icon={isEpisode ? 'tv' : 'film'} fallbackTitle={title} className={cn(blur && 'scale-110 blur-xl')} />
        <div className="absolute inset-0 flex items-center justify-center bg-black/0 transition-colors duration-300 group-hover:bg-black/30">
          <span className="flex size-11 scale-90 items-center justify-center rounded-full bg-white/90 text-black opacity-0 shadow-xl transition-all duration-300 group-hover:scale-100 group-hover:opacity-100">
            <Play className="ml-0.5 size-5 fill-current" />
          </span>
        </div>
        <ProgressBar value={progressOf(item)} />
      </button>
      <Link to={href} className="mt-2.5 block px-0.5">
        <p className="truncate text-[13px] font-medium leading-tight hover:underline">{title}</p>
        <p className="mt-0.5 truncate text-xs text-subtle">{[subtitle, left].filter(Boolean).join(' · ')}</p>
      </Link>
    </div>
  );
}

function remaining(item: Item): string {
  const pos = item.user_data?.position_ms ?? 0;
  const dur = (item.runtime_min ?? 0) * 60_000;
  if (!pos || !dur || pos >= dur) return '';
  const mins = Math.max(1, Math.round((dur - pos) / 60_000));
  return mins >= 60 ? `${Math.floor(mins / 60)}h ${mins % 60}m left` : `${mins}m left`;
}
