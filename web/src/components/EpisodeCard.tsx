import { useState, type MouseEvent } from 'react';
import { CardActions, selectCircle, type CardHover } from './CardActions';
import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { Check, Play } from 'lucide-react';
import type { Item } from '@/lib/api';
import { cn } from '@/lib/utils';
import { useSpoilerGuard } from '@/lib/home';
import { ArcArtwork } from './ArcArtwork';
import { Artwork } from './Artwork';
import { ContinueCardMenu } from './CardMenu';
import { usePlayer } from './Player';
import { episodeCode, ProgressBar, progressOf } from './WatchState';

/**
 * A landscape card for something to carry on with (Continue Watching, Up Next): clicking the
 * artwork plays it from where the profile left off. With `arc`, a backdrop card (see ArcArtwork).
 */
export function ResumeCard({ item, className, arc, onRemove, hover, onSelect, selected }: { item: Item; className?: string; arc?: boolean; onRemove?: (item: Item) => void; hover?: CardHover; onSelect?: (e: MouseEvent) => void; selected?: boolean }) {
  const { t } = useTranslation('titles');
  const [menuOpen, setMenuOpen] = useState(false);
  const play = usePlayer();
  const isEpisode = item.kind === 'episode';
  const code = episodeCode(item.season_number, item.episode_number);
  const title = isEpisode ? (item.series_title ?? item.title) : item.title;
  const subtitle = isEpisode ? code || item.title : item.year ? String(item.year) : '';
  // With spoiler protection, an episode not started yet shows its show's artwork instead of its still.
  const hideSpoilers = useSpoilerGuard()(item);
  const art = (hideSpoilers && item.images?.series_backdrop) || (item.images?.primary ?? item.images?.series_backdrop ?? item.images?.backdrop);
  const blur = hideSpoilers && art === item.images?.primary;
  const href = isEpisode ? `/series/${item.parent_id}` : `/movie/${item.id}`;
  const left = remaining(item);
  const actions = hover && !onSelect ? hover : undefined;

  return (
    <div className={cn('group relative', className)}>
      {/* The shared controls (play, ⋯ with Remove) when the card has them; else Remove on its own. */}
      {actions && <CardActions item={item} title={title} hover={actions} landscape arc={arc} menuOpen={menuOpen} onMenuOpen={setMenuOpen} onRemove={onRemove} />}
      {onRemove && !actions && <ContinueCardMenu item={item} onRemove={onRemove} />}
      <button
        type="button"
        onClick={onSelect ?? (() =>
          item.stream_url &&
          play({ title: isEpisode ? item.title : item.title, subtitle: isEpisode ? `${title} · ${code}` : subtitle, src: item.stream_url, itemId: item.id, startMS: item.user_data?.position_ms })
        )}
        className={cn("relative block w-full overflow-hidden rounded-xl shadow-lg shadow-black/25 ring-1 ring-border outline-none transition-all duration-300 ease-smooth group-hover:-translate-y-1 group-hover:shadow-2xl focus-visible:ring-2 focus-visible:ring-accent", selected && "ring-4 ring-accent")}
        aria-pressed={onSelect ? !!selected : undefined}
        aria-label={onSelect ? t('common:card.select', { title }) : t('titles:episodeCard.playTitle', { title })}
      >
        {arc ? (
          <ArcArtwork item={item} title={title} />
        ) : (
          <Artwork src={art} alt="" kind="still" icon={isEpisode ? 'tv' : 'film'} fallbackTitle={title} className={cn(blur && 'scale-110 blur-xl')} />
        )}
        {/* Its own play hint only without the shared controls, which bring their own. */}
        <div className="absolute inset-0 flex items-center justify-center bg-black/0 transition-colors duration-300 group-hover:bg-black/30">
          {!actions && (
            <span className="flex size-11 scale-90 items-center justify-center rounded-full bg-white/90 text-black opacity-0 shadow-xl transition-all duration-300 group-hover:scale-100 group-hover:opacity-100">
              <Play className="ml-0.5 size-5 fill-current" />
            </span>
          )}
        </div>
        <ProgressBar value={progressOf(item)} />
        {onSelect && <span className={cn("absolute left-2 top-2", selectCircle(!!selected))}><Check className="size-4" /></span>}
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
  return mins >= 60 ? i18n.t('titles:episodeCard.hoursLeft', { hours: Math.floor(mins / 60), minutes: mins % 60 }) : i18n.t('titles:episodeCard.minutesLeft', { count: mins });
}
