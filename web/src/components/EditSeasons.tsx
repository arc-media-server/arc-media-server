import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ChevronLeft } from 'lucide-react';
import { api, type Item, type Ordering, type Season, type SeasonArt, type SeasonsResponse } from '@/lib/api';
import { cn } from '@/lib/utils';
import { SeasonPicker, SeasonPosterThumb, SeasonPostersPanel, type ArtworkChange } from '@/components/ChooseArtwork';
import { LockToggle } from '@/components/EditDetails';
import { inputClass } from '@/components/ui/field';

type SeasonField = 'name' | 'overview';

/** A season's name and summary as edited, waiting for the Edit dialog's Save. */
export type SeasonText = { name: string; overview: string; locked: SeasonField[] };

/** Season posters belong to the aired seasons, so they can only be changed in an aired order. */
export function isAiredOrder(ordering?: Ordering | null) {
  return ordering?.source === 'tmdb_aired' || ordering?.source === 'tvdb_aired';
}

/** The fields to send for a season's edit, or nothing when it matches what's saved. */
export function seasonEdit(season: Season, text: SeasonText) {
  const edit: { name?: string; overview?: string; locked_fields: string[] } = { locked_fields: text.locked };
  if (text.name.trim() !== season.name) edit.name = text.name.trim();
  if (text.overview.trim() !== (season.overview ?? '')) edit.overview = text.overview.trim();
  return edit;
}

/**
 * The Edit dialog's Seasons tab: pick a season to change its name and summary in the episode
 * order being viewed (locked once edited, so refreshes keep them) and, in aired orders, its
 * poster. In other orders the aired seasons' posters are listed underneath.
 */
export function SeasonsPanel({
  item,
  view,
  error,
  art,
  onArt,
  text,
  onText,
}: {
  item: Item;
  /** The show's seasons in the order being viewed. */
  view?: SeasonsResponse;
  error?: Error | null;
  art: Record<number, ArtworkChange>;
  onArt: (season: number, change: ArtworkChange | undefined) => void;
  text: Record<number, SeasonText>;
  onText: (season: number, text: SeasonText | undefined) => void;
}) {
  const { t } = useTranslation('details');
  const aired = isAiredOrder(view?.ordering);
  const posters = useQuery({ queryKey: ['artwork', item.id, 'seasons'], queryFn: () => api.seasonArt(item.id), enabled: aired });
  const [open, setOpen] = useState<number | null>(null);
  const season = view?.seasons.find((s) => s.season_number === open);
  const posterOf = (n: number): SeasonArt | undefined => posters.data?.seasons.find((s) => s.season_number === n);

  if (season) {
    const poster = aired ? posterOf(season.season_number) : undefined;
    return (
      <div>
        <button type="button" onClick={() => setOpen(null)} className="mb-3 flex items-center gap-1 text-[13px] font-medium text-muted hover:text-foreground">
          <ChevronLeft className="size-4" /> {t('details:editSeasons.allSeasons')}</button>
        <SeasonTextFields season={season} text={text[season.season_number]} onChange={(v) => onText(season.season_number, v)} />
        {aired && (
          <div className="mt-6 border-t border-border pt-5">
            <h3 className="mb-3 text-[11px] font-semibold uppercase tracking-wider text-subtle">{t('details:editSeasons.poster')}</h3>
            <SeasonPicker
              item={item}
              season={{ season_number: season.season_number, name: season.name, chosen: poster?.chosen ?? false, local: poster?.local ?? false }}
              change={art[season.season_number]}
              onChange={(c) => onArt(season.season_number, c)}
            />
          </div>
        )}
      </div>
    );
  }
  return (
    <div>
      <p className="mb-4 text-[13px] text-muted">
        {aired
          ? t('details:editSeasons.pickAired', { order: view?.ordering?.name })
          : t('details:editSeasons.pick', { order: view?.ordering?.name })}
      </p>
      {error && <p className="text-sm text-warning">{error.message}</p>}
      {!view && !error && <div className="h-64 animate-pulse rounded-2xl bg-surface-2" />}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-3 pb-2">
        {view?.seasons.map((s) => {
          const changed = !!text[s.season_number] || (aired && !!art[s.season_number]);
          return (
            <button key={s.season_number} type="button" onClick={() => setOpen(s.season_number)} className="group text-left">
              <div className="relative aspect-[2/3] overflow-hidden rounded-xl bg-surface-2 ring-1 ring-border transition-all group-hover:-translate-y-0.5 group-hover:ring-2 group-hover:ring-accent/60">
                {aired ? (
                  <SeasonPosterThumb seriesId={item.id} season={s.season_number} image={posterOf(s.season_number)?.image ?? s.poster} change={art[s.season_number]} />
                ) : s.poster ? (
                  <img src={s.poster} alt="" loading="lazy" className="size-full object-cover" />
                ) : null}
                {changed && <span className="absolute right-1.5 top-1.5 size-2.5 rounded-full bg-accent shadow" aria-label={t('details:editSeasons.changed')} />}
              </div>
              <p className="mt-1.5 truncate text-[13px] font-medium">{text[s.season_number]?.name.trim() || s.name}</p>
              <p className="text-xs text-subtle">
                {changed ? t('details:editSeasons.changedLabel') : t('details:editSeasons.episodes', { count: s.episodes.length })}
              </p>
            </button>
          );
        })}
      </div>
      {view && !aired && (
        <div className="mt-6 border-t border-border pt-5">
          <h3 className="mb-1 text-[11px] font-semibold uppercase tracking-wider text-subtle">{t('details:editSeasons.airedPosters')}</h3>
          <SeasonPostersPanel item={item} changes={art} onChange={onArt} />
        </div>
      )}
    </div>
  );
}

/** A season's name and summary, each locked once edited. */
function SeasonTextFields({ season, text, onChange }: { season: Season; text?: SeasonText; onChange: (text: SeasonText | undefined) => void }) {
  const { t } = useTranslation('details');
  const current: SeasonText = text ?? { name: season.name, overview: season.overview ?? '', locked: season.locked_fields };
  const update = (next: SeasonText) => {
    const same =
      next.name === season.name &&
      next.overview === (season.overview ?? '') &&
      next.locked.length === season.locked_fields.length &&
      next.locked.every((f) => season.locked_fields.includes(f));
    onChange(same ? undefined : next);
  };
  const edit = (field: SeasonField, value: string) =>
    update({ ...current, [field]: value, locked: current.locked.includes(field) ? current.locked : [...current.locked, field] });
  const toggle = (field: SeasonField) =>
    update({ ...current, locked: current.locked.includes(field) ? current.locked.filter((f) => f !== field) : [...current.locked, field] });

  return (
    <div className="space-y-4">
      <p className="text-[13px] text-muted">{t('details:editSeasons.locked')}</p>
      <label className="block">
        <span className="mb-1.5 flex items-center justify-between">
          <span className="text-[11px] font-semibold uppercase tracking-wider text-subtle">{t('details:editSeasons.name')}</span>
          <LockToggle locked={current.locked.includes('name')} onToggle={() => toggle('name')} />
        </span>
        <input aria-label={t('details:editSeasons.name')} value={current.name} onChange={(e) => edit('name', e.target.value)} className={inputClass} aria-invalid={!current.name.trim()} />
      </label>
      <label className="block">
        <span className="mb-1.5 flex items-center justify-between">
          <span className="text-[11px] font-semibold uppercase tracking-wider text-subtle">{t('details:editSeasons.summary')}</span>
          <LockToggle locked={current.locked.includes('overview')} onToggle={() => toggle('overview')} />
        </span>
        <textarea
          aria-label={t('details:editSeasons.summary')}
          value={current.overview}
          onChange={(e) => edit('overview', e.target.value)}
          rows={5}
          placeholder={t('details:editSeasons.noSummaryYet')}
          className={cn(inputClass, 'h-auto resize-y py-2.5 leading-relaxed')}
        />
      </label>
    </div>
  );
}
