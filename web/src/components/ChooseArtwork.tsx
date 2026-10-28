import { useTranslation } from 'react-i18next';
import { useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, ChevronLeft, RotateCcw, Upload } from 'lucide-react';
import { api, type ArtworkChoices, type ArtworkImage, type ArtworkType, type Item } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';


/** An artwork change waiting to be saved: a provider image, an upload, or back to automatic. */
export type ArtworkChange = { path: string } | { file: File; preview: string } | { automatic: true };

/** Saves an artwork change. */
export function applyArtwork(itemId: string, type: ArtworkType, change: ArtworkChange) {
  if ('file' in change) return api.uploadArtwork(itemId, type, change.file);
  return api.chooseArtwork(itemId, type, 'path' in change ? change.path : null);
}

/**
 * Pick a poster, background or logo from TMDB's, upload one, or go back to automatic. The choice
 * is only shown as pending; the Edit dialog saves it.
 */
export function ArtworkPanel({
  item,
  type,
  change,
  onChange,
  source,
}: {
  item: Pick<Item, 'id' | 'kind'>;
  type: ArtworkType;
  change?: ArtworkChange;
  onChange: (change: ArtworkChange | undefined) => void;
  /** Where the choices come from, when not a title's own (e.g. a collection's). */
  source?: { key: unknown[]; load: () => Promise<ArtworkChoices> };
}) {
  const { t } = useTranslation('metadata');
  const file = useRef<HTMLInputElement>(null);
  const choices = useQuery({ queryKey: source?.key ?? ['artwork', item.id], queryFn: source?.load ?? (() => api.artwork(item.id)) });
  const c = choices.data;
  const images = type === 'poster' ? c?.posters : type === 'logo' ? c?.logos : c?.backdrops;
  const chosen = c?.chosen[type];
  // What's selected: the pending pick, or what's in use now.
  const selected = change && 'path' in change ? change.path : change ? undefined : c?.current[type];
  const tile = item.kind === 'episode' ? 'backdrop' : type;
  const messages = {
    poster: {
      automatic: t('metadata:chooseArtwork.posterAutomatic'),
      upload: t('metadata:chooseArtwork.posterUpload'),
      local: t('metadata:chooseArtwork.posterLocal'),
      empty: t('metadata:chooseArtwork.posterEmpty'),
    },
    backdrop: {
      automatic: t('metadata:chooseArtwork.backdropAutomatic'),
      upload: t('metadata:chooseArtwork.backdropUpload'),
      local: t('metadata:chooseArtwork.backdropLocal'),
      empty: t('metadata:chooseArtwork.backdropEmpty'),
    },
    logo: {
      automatic: t('metadata:chooseArtwork.logoAutomatic'),
      upload: t('metadata:chooseArtwork.logoUpload'),
      local: t('metadata:chooseArtwork.logoLocal'),
      empty: t('metadata:chooseArtwork.logoEmpty'),
    },
    thumbnail: {
      automatic: t('metadata:chooseArtwork.thumbnailAutomatic'),
      upload: t('metadata:chooseArtwork.thumbnailUpload'),
      local: t('metadata:chooseArtwork.thumbnailLocal'),
      empty: t('metadata:chooseArtwork.thumbnailEmpty'),
    },
  };
  const message = messages[item.kind === 'episode' ? 'thumbnail' : type];
  let note = t('metadata:chooseArtwork.description');
  if (change && 'automatic' in change) note = message.automatic;
  else if (change && 'file' in change) note = message.upload;
  else if (c?.local[type] && !chosen) note = message.local;

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <p className="mr-auto max-w-md text-[13px] text-muted">{note}</p>
        {(chosen || change) && !(change && 'automatic' in change) && (
          <Button variant="ghost" size="sm" onClick={() => onChange(chosen ? { automatic: true } : undefined)}>
            <RotateCcw /> {t("metadata:chooseArtwork.automatic")}</Button>
        )}
        <input
          ref={file}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          hidden
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) onChange({ file: f, preview: URL.createObjectURL(f) });
            e.target.value = '';
          }}
        />
        <Button variant="secondary" size="sm" onClick={() => file.current?.click()}>
          <Upload /> {t("metadata:chooseArtwork.upload")}</Button>
      </div>
      {choices.error && <p className="text-sm text-warning">{choices.error.message}</p>}
      {!c && !choices.error && <div className="h-64 animate-pulse rounded-2xl bg-surface-2" />}
      {images?.length === 0 && !(change && 'file' in change) && (
        <p className="py-10 text-center text-sm text-muted">{message.empty}</p>
      )}
      <div className={cn('grid gap-3 pb-2', tile === 'poster' ? 'grid-cols-[repeat(auto-fill,minmax(120px,1fr))]' : 'grid-cols-[repeat(auto-fill,minmax(220px,1fr))]')}>
        {change && 'file' in change && <Tile type={tile} src={change.preview} selected title={t("metadata:chooseArtwork.yourUpload")} onClick={() => undefined} />}
        {images?.map((img) => (
          <Tile
            key={img.path}
            type={tile}
            src={img.thumb}
            selected={img.path === selected}
            title={`${img.width}×${img.height}${img.language ? ` · ${img.language}` : ''}`}
            onClick={() => onChange(img.path === c?.current[type] && !chosen ? undefined : { path: img.path })}
          />
        ))}
      </div>
    </div>
  );
}

function Tile({ type, src, selected, title, onClick }: { type: ArtworkType; src: string; selected: boolean; title: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'group relative overflow-hidden rounded-xl bg-surface-2 ring-1 ring-border transition-all hover:-translate-y-0.5 hover:ring-2 hover:ring-accent/60',
        type === 'poster' ? 'aspect-[2/3]' : 'aspect-video',
        type === 'logo' && 'bg-[repeating-conic-gradient(var(--color-surface-2)_0_25%,var(--color-surface)_0_50%)] bg-[length:16px_16px] p-4',
        selected && 'ring-2 ring-accent',
      )}
      title={title}
    >
      <img src={src} alt="" loading="lazy" className={cn('size-full', type === 'logo' ? 'object-contain' : 'object-cover')} />
      {selected && (
        <span className="absolute right-1.5 top-1.5 flex size-6 items-center justify-center rounded-full bg-accent-strong text-white shadow">
          <Check className="size-3.5" strokeWidth={3} />
        </span>
      )}
    </button>
  );
}

/** Saves a season poster change. */
export function applySeasonArtwork(seriesId: string, season: number, change: ArtworkChange) {
  if ('file' in change) return api.uploadSeasonArtwork(seriesId, season, change.file);
  return api.chooseSeasonArtwork(seriesId, season, 'path' in change ? change.path : null);
}

/**
 * A show's aired seasons with their posters; pick one to choose its poster from TMDB's, upload
 * one, or go back to automatic. Changes wait for the Edit dialog's Save.
 */
export function SeasonPostersPanel({
  item,
  changes,
  onChange,
}: {
  item: Item;
  changes: Record<number, ArtworkChange>;
  onChange: (season: number, change: ArtworkChange | undefined) => void;
}) {
  const { t } = useTranslation('metadata');
  const seasons = useQuery({ queryKey: ['artwork', item.id, 'seasons'], queryFn: () => api.seasonArt(item.id) });
  const [open, setOpen] = useState<number | null>(null);
  const season = seasons.data?.seasons.find((s) => s.season_number === open);

  if (season) {
    return (
      <div>
        <button type="button" onClick={() => setOpen(null)} className="mb-3 flex items-center gap-1 text-[13px] font-medium text-muted hover:text-foreground">
          <ChevronLeft className="size-4" /> {t("metadata:chooseArtwork.allSeasons")}</button>
        <SeasonPicker item={item} season={season} change={changes[season.season_number]} onChange={(c) => onChange(season.season_number, c)} />
      </div>
    );
  }
  return (
    <div>
      <p className="mb-4 text-[13px] text-muted">{t("metadata:chooseArtwork.seasonPostersShowOn")}</p>
      {seasons.error && <p className="text-sm text-warning">{seasons.error.message}</p>}
      {!seasons.data && !seasons.error && <div className="h-64 animate-pulse rounded-2xl bg-surface-2" />}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-3 pb-2">
        {seasons.data?.seasons.map((s) => {
          const c = changes[s.season_number];
          return (
            <button key={s.season_number} type="button" onClick={() => setOpen(s.season_number)} className="group text-left">
              <div className="relative aspect-[2/3] overflow-hidden rounded-xl bg-surface-2 ring-1 ring-border transition-all group-hover:-translate-y-0.5 group-hover:ring-2 group-hover:ring-accent/60">
                <SeasonPosterThumb seriesId={item.id} season={s.season_number} image={s.image} change={c} />
                {c && <span className="absolute right-1.5 top-1.5 size-2.5 rounded-full bg-accent shadow" aria-label={t("metadata:chooseArtwork.changed")} />}
              </div>
              <p className="mt-1.5 truncate text-[13px] font-medium">{s.name}</p>
              <p className="text-xs text-subtle">{c ? t("metadata:chooseArtwork.changed2") : s.chosen ? t("metadata:chooseArtwork.chosen") : s.local ? t("metadata:chooseArtwork.fromFolder") : s.image ? t("metadata:chooseArtwork.automatic") : t("metadata:chooseArtwork.noneYet")}</p>
            </button>
          );
        })}
      </div>
    </div>
  );
}

/** A season's poster tile image: a pending change if there is one, else the poster in use. */
export function SeasonPosterThumb({ seriesId, season, image, change }: { seriesId: string; season: number; image?: string; change?: ArtworkChange }) {
  const { t } = useTranslation('metadata');
  const src = change ? ('file' in change ? change.preview : undefined) : image;
  if (src) return <img src={src} alt="" loading="lazy" className="size-full object-cover" />;
  if (change && 'path' in change) return <PendingThumb seriesId={seriesId} season={season} path={change.path} />;
  return (
    <span className="flex size-full items-center justify-center p-2 text-center text-xs text-subtle">
      {change && 'automatic' in change ? t("metadata:chooseArtwork.automatic") : t("metadata:chooseArtwork.showPoster")}
    </span>
  );
}

/** The thumbnail of a TMDB poster picked for a season, looked up from the season's choices. */
function PendingThumb({ seriesId, season, path }: { seriesId: string; season: number; path: string }) {
  const choices = useQuery({ queryKey: ['artwork', seriesId, 'season', season], queryFn: () => api.seasonArtwork(seriesId, season) });
  const img = choices.data?.posters.find((p) => p.path === path);
  return img ? <img src={img.thumb} alt="" className="size-full object-cover" /> : null;
}

export function SeasonPicker({
  item,
  season,
  change,
  onChange,
}: {
  item: Item;
  season: { season_number: number; name: string; chosen: boolean; local: boolean };
  change?: ArtworkChange;
  onChange: (change: ArtworkChange | undefined) => void;
}) {
  const { t } = useTranslation('metadata');
  const file = useRef<HTMLInputElement>(null);
  const choices = useQuery({
    queryKey: ['artwork', item.id, 'season', season.season_number],
    queryFn: () => api.seasonArtwork(item.id, season.season_number),
  });
  const posters: ArtworkImage[] | undefined = choices.data?.posters;
  const selected = change && 'path' in change ? change.path : undefined;

  let note = t('metadata:chooseArtwork.seasonDescription', { season: season.name });
  if (change && 'automatic' in change) note = t('metadata:chooseArtwork.seasonAutomatic', { season: season.name });
  else if (change && 'file' in change) note = t('metadata:chooseArtwork.seasonUpload');
  else if (season.local && !season.chosen) note = t('metadata:chooseArtwork.seasonLocal');

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <p className="mr-auto max-w-md text-[13px] text-muted">{note}</p>
        {(season.chosen || change) && !(change && 'automatic' in change) && (
          <Button variant="ghost" size="sm" onClick={() => onChange(season.chosen ? { automatic: true } : undefined)}>
            <RotateCcw /> {t("metadata:chooseArtwork.automatic")}</Button>
        )}
        <input
          ref={file}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          hidden
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) onChange({ file: f, preview: URL.createObjectURL(f) });
            e.target.value = '';
          }}
        />
        <Button variant="secondary" size="sm" onClick={() => file.current?.click()}>
          <Upload /> {t("metadata:chooseArtwork.upload")}</Button>
      </div>
      {choices.error && <p className="text-sm text-warning">{choices.error.message}</p>}
      {!posters && !choices.error && <div className="h-64 animate-pulse rounded-2xl bg-surface-2" />}
      {posters?.length === 0 && !(change && 'file' in change) && (
        <p className="py-10 text-center text-sm text-muted">{t('metadata:chooseArtwork.noSeasonPosters', { name: season.name })}</p>
      )}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-3 pb-2">
        {change && 'file' in change && <Tile type="poster" src={change.preview} selected title={t("metadata:chooseArtwork.yourUpload")} onClick={() => undefined} />}
        {posters?.map((img) => (
          <Tile
            key={img.path}
            type="poster"
            src={img.thumb}
            selected={img.path === selected}
            title={`${img.width}×${img.height}${img.language ? ` · ${img.language}` : ''}`}
            onClick={() => onChange(img.path === selected ? undefined : { path: img.path })}
          />
        ))}
      </div>
    </div>
  );
}
