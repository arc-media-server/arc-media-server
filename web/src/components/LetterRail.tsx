import { useTranslation } from 'react-i18next';
import type { LetterCount } from '@/lib/api';
import { cn } from '@/lib/utils';

const rail = ['#', ...'abcdefghijklmnopqrstuvwxyz'];

/**
 * Where each letter starts in a title-sorted list: the offset of its first title in the list's
 * direction. Letters with no titles are left out.
 */
export function letterOffsets(letters: LetterCount[], reverse: boolean) {
  const out = new Map<string, number>();
  let at = 0;
  for (const { letter, count } of reverse ? [...letters].reverse() : letters) {
    if (!out.has(letter)) out.set(letter, at);
    at += count;
  }
  return out;
}

/**
 * The A–Z rail beside a title-sorted library, as Plex's: choosing a letter jumps to its first
 * title in the whole list. Letters with no titles are dimmed.
 */
export function LetterRail({ offsets, current, onJump, className }: {
  offsets: Map<string, number>;
  current?: string;
  onJump: (letter: string, offset: number) => void;
  className?: string;
}) {
  const { t } = useTranslation('library');
  return (
    <nav aria-label={t('library:letterRail')} className={cn('flex flex-col items-center justify-between rounded-full bg-surface/60 px-0.5 py-1.5 ring-1 ring-border', className)}>
      {rail.map((letter) => {
        const offset = offsets.get(letter);
        const label = letter === '#' ? t('library:letterOther') : letter.toUpperCase();
        return (
          <button
            key={letter}
            type="button"
            disabled={offset === undefined}
            aria-current={current === letter ? 'true' : undefined}
            aria-label={t('library:jumpToLetter', { letter: label })}
            title={label}
            onClick={() => offset !== undefined && onJump(letter, offset)}
            className={cn(
              'flex size-6 shrink items-center justify-center rounded-full text-[10.5px] font-semibold leading-none transition-colors',
              offset === undefined ? 'cursor-default text-subtle/40' : 'text-muted hover:bg-surface-2 hover:text-foreground',
              current === letter && 'bg-foreground text-background hover:bg-foreground hover:text-background',
            )}
          >
            {letter.toUpperCase()}
          </button>
        );
      })}
    </nav>
  );
}
