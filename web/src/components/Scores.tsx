import { useTranslation } from 'react-i18next';
import { number } from '@/lib/format';
import type { Item } from '@/lib/api';
import { cn } from '@/lib/utils';

/**
 * A title's scores from other sites: IMDb, Rotten Tomatoes' critics and Metacritic, as Plex shows
 * them. TMDB's audience score stands in when there are none. Returns null when there's nothing.
 */
export function Scores({ item }: { item: Item }) {
  const { t } = useTranslation('titles');
  const rt = item.rotten_tomatoes;
  const mc = item.metacritic;
  const imdb = item.imdb_rating;
  if (!imdb && rt == null && mc == null) {
    return item.rating ? <span title={t('titles:scores.audience')}>★ {item.rating.toFixed(1)}</span> : null;
  }
  return (
    <span className="flex items-center gap-3">
      {!!imdb && (
        <span className="flex items-center gap-1.5" title={item.imdb_votes ? t('titles:scores.votes', { count: item.imdb_votes, value: number(item.imdb_votes) }) : 'IMDb'}>
          <span className="rounded-[3px] bg-[#f5c518] px-1 text-[10px] font-black leading-[15px] tracking-tight text-black">IMDb</span>
          <span className="text-foreground/85">{imdb.toFixed(1)}</span>
        </span>
      )}
      {rt != null && (
        <span className="flex items-center gap-1.5" title={rt >= 60 ? t('titles:scores.fresh') : t('titles:scores.rotten')}>
          <Tomato fresh={rt >= 60} />
          <span className="text-foreground/85">{rt}%</span>
        </span>
      )}
      {mc != null && (
        <span className="flex items-center gap-1.5" title="Metacritic">
          <span
            className={cn(
              'flex h-[17px] min-w-[20px] items-center justify-center rounded-[3px] px-0.5 text-[11px] font-bold text-white',
              mc >= 61 ? 'bg-[#00ce7a]' : mc >= 40 ? 'bg-[#ffbd3f] text-black' : 'bg-[#ff6874]',
            )}
          >
            {mc}
          </span>
        </span>
      )}
    </span>
  );
}

/** Rotten Tomatoes' symbols: a tomato when fresh, a green splat when rotten. */
function Tomato({ fresh }: { fresh: boolean }) {
  return fresh ? (
    <svg viewBox="0 0 16 16" className="size-4" aria-hidden>
      <circle cx="8" cy="9" r="6" fill="#fa320a" />
      <path d="M8 3.2c-.6-1.2-1.8-1.6-3-1.3 1 .3 1.6.9 1.9 1.6-1.2-.2-2.3.2-3 1 1.2-.3 2.4 0 3.1.6L8 4.2l1 .9c.7-.6 1.9-.9 3.1-.6-.7-.8-1.8-1.2-3-1 .3-.7.9-1.3 1.9-1.6-1.2-.3-2.4.1-3 1.3Z" fill="#00912d" />
    </svg>
  ) : (
    <svg viewBox="0 0 16 16" className="size-4" aria-hidden>
      <path
        d="M8 2.5 9.4 5l2.8-1-.6 2.9 2.8 1.2-2.4 1.7 1.3 2.6-2.9-.3L9.6 15 8 12.6 6.4 15l-.8-2.9-2.9.3L4 9.8 1.6 8.1l2.8-1.2L3.8 4l2.8 1Z"
        fill="#0fc755"
      />
    </svg>
  );
}
