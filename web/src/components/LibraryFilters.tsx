import { useTranslation } from 'react-i18next';
import { Popover } from 'radix-ui';
import { SlidersHorizontal, X } from 'lucide-react';
import type { LibraryFacets, Resolution } from '@/lib/api';
import { language } from '@/lib/format';
import { addedOptions, getLengthOptions, ratingOptions } from '@/components/FilterFields';
import { Button } from '@/components/ui/button';
import { Field, inputClass } from '@/components/ui/field';
import { Switch } from '@/components/ui/switch';

const resolutionLabels: Record<Resolution, string> = { '4k': '4K', '1080': '1080p', '720': '720p', sd: 'SD' };

/** A library's browse filters, as they are in the address. */
export interface BrowseFilters {
  genre?: string;
  language?: string;
  label?: string;
  decade?: number;
  resolution?: Resolution;
  unwatched?: boolean;
  added_within?: number;
  min_runtime?: number;
  max_runtime?: number;
  min_rating?: number;
}

/** Changes to the address: a value to set, or undefined to remove it. */
export type FilterPatch = Partial<Record<keyof BrowseFilters, string | undefined>>;

type Choice = { id: string; label: string; anyLabel: string; value?: string; options: [value: string, label: string, count?: number][]; patch: (v?: string) => FilterPatch };

/** The filters that apply to a library, each with its choices and what's chosen. */
function useChoices(facets: LibraryFacets | undefined, f: BrowseFilters): Choice[] {
  const { t } = useTranslation('library');
  if (!facets) return [];
  const choices: Choice[] = [
    { id: 'genre', label: t('filters:genre.label'), anyLabel: t('filters:genre.any'), value: f.genre,
      options: facets.genres.map((x) => [x.value, x.value, x.count]), patch: (v) => ({ genre: v }) },
  ];
  if (facets.labels.length || f.label) choices.push({ id: 'label', label: t('filters:label.label'), anyLabel: t('filters:label.any'), value: f.label,
    options: facets.labels.map((x) => [x.value, x.value, x.count]), patch: (v) => ({ label: v }) });
  if (facets.languages.length > 1 || f.language) choices.push({ id: 'language', label: t('library:language'), anyLabel: t('filters:language.any'), value: f.language,
    options: facets.languages.map((x) => [x.value, language(x.value), x.count]), patch: (v) => ({ language: v }) });
  choices.push({ id: 'decade', label: t('filters:decade.label'), anyLabel: t('filters:decade.any'), value: f.decade ? String(f.decade) : undefined,
    options: facets.decades.map((x) => [x.value, t('filters:decade.value', { year: x.value }), x.count]), patch: (v) => ({ decade: v }) });
  if (facets.resolutions.length > 1 || f.resolution) choices.push({ id: 'resolution', label: t('filters:quality.label'), anyLabel: t('filters:quality.any'), value: f.resolution,
    options: facets.resolutions.map((x) => [x.value, resolutionLabels[x.value as Resolution] ?? x.value, x.count]), patch: (v) => ({ resolution: v }) });
  choices.push(
    { id: 'added', label: t('filters:added.label'), anyLabel: t('filters:added.any'), value: f.added_within ? String(f.added_within) : undefined,
      options: addedOptions.filter(Boolean).map((d) => [String(d), d === 7 ? t('filters:added.week') : d === 365 ? t('filters:added.year') : t('filters:added.lastDays', { count: d })]),
      patch: (v) => ({ added_within: v }) },
    { id: 'length', label: t('filters:length.label'), anyLabel: t('filters:length.any'),
      value: f.min_runtime || f.max_runtime ? `${f.min_runtime ?? ''}-${f.max_runtime ?? ''}` : undefined,
      options: getLengthOptions().slice(1).map((o) => [`${o.min ?? ''}-${o.max ?? ''}`, o.label]),
      patch: (v) => { const [min, max] = (v ?? '-').split('-'); return { min_runtime: min || undefined, max_runtime: max || undefined }; } },
    { id: 'rating', label: t('filters:rating.label'), anyLabel: t('filters:rating.any'), value: f.min_rating ? String(f.min_rating) : undefined,
      options: ratingOptions.filter(Boolean).map((r) => [String(r), t('filters:rating.atLeast', { rating: r })]), patch: (v) => ({ min_rating: v }) },
  );
  return choices;
}

/**
 * The Filters button: every filter in one panel, so the bar stays short. It shows how many are
 * on; the chosen ones also show beside it (ActiveFilters) to remove one at a time.
 */
export function FiltersButton({ facets, filters, onChange, onClear }: { facets?: LibraryFacets; filters: BrowseFilters; onChange: (patch: FilterPatch) => void; onClear: () => void }) {
  const { t } = useTranslation('library');
  const choices = useChoices(facets, filters);
  const count = choices.filter((c) => c.value).length + (filters.unwatched ? 1 : 0);
  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <Button variant="secondary" size="sm" className="shrink-0" aria-label={count ? t('library:filtersOn', { count }) : t('library:filters')}>
          <SlidersHorizontal /> {t('library:filters')}
          {count > 0 && <span className="ml-0.5 flex size-5 items-center justify-center rounded-full bg-accent-strong text-[11px] font-semibold text-white">{count}</span>}
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={8}
          collisionPadding={12}
          aria-label={t('library:filters')}
          className="animate-menu-in z-50 w-[min(560px,calc(100vw-24px))] rounded-2xl border border-border bg-surface p-5 shadow-2xl shadow-black/40"
        >
          <label className="mb-4 flex items-center justify-between gap-4 rounded-xl bg-surface-2/60 px-3.5 py-2.5 text-sm font-medium">
            {t('library:unwatchedOnly')}
            <Switch checked={!!filters.unwatched} onCheckedChange={(on) => onChange({ unwatched: on ? '1' : undefined })} />
          </label>
          <div className="grid gap-x-4 gap-y-3 sm:grid-cols-2">
            {choices.map((c) => (
              <Field key={c.id} label={c.label}>
                <select value={c.value ?? ''} onChange={(e) => onChange(c.patch(e.target.value || undefined))} className={inputClass}>
                  <option value="">{c.anyLabel}</option>
                  {c.options.map(([v, l, n]) => <option key={v} value={v}>{n === undefined ? l : `${l} (${n})`}</option>)}
                </select>
              </Field>
            ))}
          </div>
          {count > 0 && (
            <div className="mt-4 flex justify-end">
              <Button variant="ghost" size="sm" onClick={onClear}><X /> {t('library:clearFilters')}</Button>
            </div>
          )}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

/** The filters that are on, as chips that each turn theirs off. */
export function ActiveFilters({ facets, filters, onChange }: { facets?: LibraryFacets; filters: BrowseFilters; onChange: (patch: FilterPatch) => void }) {
  const { t } = useTranslation('library');
  const on = useChoices(facets, filters).filter((c) => c.value);
  const chips: { key: string; text: string; patch: FilterPatch }[] = [
    ...(filters.unwatched ? [{ key: 'unwatched', text: t('library:unwatched'), patch: { unwatched: undefined } }] : []),
    ...on.map((c) => ({ key: c.id, text: c.options.find(([v]) => v === c.value)?.[1] ?? c.value!, patch: c.patch(undefined) })),
  ];
  return (
    <>
      {chips.map((c) => (
        <button
          key={c.key}
          type="button"
          onClick={() => onChange(c.patch)}
          aria-label={t('library:removeFilter', { filter: c.text })}
          className="flex h-8 max-w-56 shrink-0 items-center gap-1.5 rounded-full bg-foreground pl-3.5 pr-2.5 text-[13px] font-medium text-background transition-opacity hover:opacity-85"
        >
          <span className="truncate">{c.text}</span> <X className="size-3.5 opacity-70" />
        </button>
      ))}
    </>
  );
}
