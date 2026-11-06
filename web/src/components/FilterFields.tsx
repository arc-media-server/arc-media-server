import { sortLabels } from '@/lib/filter-labels';
import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { api, type DynamicRange, type Facet, type ItemSort, type Library, type Resolution, type RowFilter, type WatchState } from '@/lib/api';
import { channelsLabel, codecLabel, dynamicRangeLabel } from '@/lib/media-labels';
import { rowQuery } from '@/lib/home';
import * as fmt from '@/lib/format';
import { resolutions } from '@/pages/Library';
import { ErrorNotice } from './Page';
import { Artwork } from '@/components/Artwork';
import { Field, inputClass } from '@/components/ui/field';

export const addedOptions = [0, 7, 30, 90, 365];
export function getLengthOptions(): { label: string; min?: number; max?: number }[] {
  return [
  { label: i18n.t('filters:length.any') },
  { label: i18n.t('filters:length.underNinety'), max: 90 },
  { label: i18n.t('filters:length.underTwoHours'), max: 120 },
  { label: i18n.t('filters:length.overTwoHours'), min: 120 },
  { label: i18n.t('filters:length.overTwoAndHalfHours'), min: 150 },
  ];
}
export const ratingOptions = [0, 6, 7, 8];

/**
 * The filters a custom home row or smart collection is made of. With libraries, the library can
 * be chosen too.
 */
export function FilterFields({
  filter: f,
  onChange,
  libraries,
  unwatchedLabel,
}: {
  filter: RowFilter;
  onChange: (f: RowFilter) => void;
  libraries?: Library[];
  unwatchedLabel?: string;
}) {
  const { t } = useTranslation('filters');
  const facets = useQuery({ queryKey: ['facets', f.library_id], queryFn: () => api.facets(f.library_id), enabled: !!f.library_id });
  const library = libraries?.find((l) => l.id === f.library_id);
  const movies = library ? library.kind !== 'shows' : true;
  const set = (patch: Partial<RowFilter>) => onChange({ ...f, ...patch });

  return (
    <>
      <ErrorNotice error={facets.error} onRetry={() => void facets.refetch()} />
      <div className="grid gap-3 sm:grid-cols-2">
        {libraries && (
          <Field label={t('library')}>
            <select
              value={f.library_id}
              onChange={(e) => onChange({ library_id: Number(e.target.value), sort: f.sort, unwatched: f.unwatched })}
              className={inputClass}
            >
              {libraries.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </select>
          </Field>
        )}
        <Field label={t('order')}>
          <select value={f.sort && f.sort in sortLabels() ? f.sort : 'title'} onChange={(e) => set({ sort: e.target.value as ItemSort })} className={inputClass}>
            {Object.entries(sortLabels()).map(([k, label]) => (
              <option key={k} value={k}>
                {label}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t('genre.label')}>
          <select value={f.genre ?? ''} onChange={(e) => set({ genre: e.target.value || undefined })} className={inputClass}>
            <option value="">{t('genre.any')}</option>
            {facets.data?.genres.map((g) => (
              <option key={g.value} value={g.value}>
                {g.value}
              </option>
            ))}
          </select>
        </Field>
        {(facets.data?.labels.length ?? 0) > 0 && (
          <Field label={t('label.label')}>
            <select value={f.label ?? ''} onChange={(e) => set({ label: e.target.value || undefined })} className={inputClass}>
              <option value="">{t('label.any')}</option>
              {facets.data?.labels.map((l) => (
                <option key={l.value} value={l.value}>
                  {l.value}
                </option>
              ))}
            </select>
          </Field>
        )}
        {(facets.data?.languages.length ?? 0) > 1 && (
          <Field label={t('language.label')}>
            <select value={f.language ?? ''} onChange={(e) => set({ language: e.target.value || undefined })} className={inputClass}>
              <option value="">{t('language.any')}</option>
              {facets.data?.languages.map((l) => (
                <option key={l.value} value={l.value}>
                  {fmt.language(l.value)}
                </option>
              ))}
            </select>
          </Field>
        )}
        <Field label={t('decade.label')}>
          <select value={f.decade ?? ''} onChange={(e) => set({ decade: Number(e.target.value) || undefined })} className={inputClass}>
            <option value="">{t('decade.any')}</option>
            {facets.data?.decades.map((d) => (
              <option key={d.value} value={d.value}>
                {t('decade.value', { year: d.value })}
              </option>
            ))}
          </select>
        </Field>
        <AdvancedFilterFields filter={f} onChange={onChange} />
        <Field label={t('watchState.label')}>
          <select value={f.watch_state ?? ''} onChange={(e) => set({ watch_state: (e.target.value as WatchState) || undefined, ...(e.target.value ? { unwatched: undefined } : {}) })} className={inputClass}>
            <option value="">{t('watchState.any')}</option>
            <option value="played">{t('watchState.played')}</option>
            <option value="unstarted">{t('watchState.unstarted')}</option>
          </select>
        </Field>
        <FacetSelect label={t('dynamicRange.label')} any={t('dynamicRange.any')} values={facets.data?.dynamic_ranges} value={f.dynamic_range} name={dynamicRangeLabel}
          onChange={(v) => set({ dynamic_range: (v as DynamicRange) || undefined })} />
        <FacetSelect label={t('videoCodec.label')} any={t('videoCodec.any')} values={facets.data?.video_codecs} value={f.video_codec} name={codecLabel}
          onChange={(v) => set({ video_codec: v || undefined })} />
        <FacetSelect label={t('audioCodec.label')} any={t('audioCodec.any')} values={facets.data?.audio_codecs} value={f.audio_codec} name={codecLabel}
          onChange={(v) => set({ audio_codec: v || undefined })} />
        <FacetSelect label={t('channels.label')} any={t('channels.any')} values={facets.data?.audio_channels && [...facets.data.audio_channels].sort((a, b) => Number(b.value) - Number(a.value))} value={f.audio_channels ? String(f.audio_channels) : undefined} name={channelsLabel}
          onChange={(v) => set({ audio_channels: Number(v) || undefined })} />
        {movies && (
          <Field label={t('quality.label')}>
            <select value={f.resolution ?? ''} onChange={(e) => set({ resolution: (e.target.value as Resolution) || undefined })} className={inputClass}>
              <option value="">{t('quality.any')}</option>
              {Object.entries(resolutions).map(([k, label]) => (
                <option key={k} value={k}>
                  {label}
                </option>
              ))}
            </select>
          </Field>
        )}
      </div>
      <label className="flex cursor-pointer items-center gap-2.5 text-sm">
        <input type="checkbox" checked={!!f.unwatched} onChange={(e) => set({ unwatched: e.target.checked || undefined, ...(e.target.checked ? { watch_state: undefined } : {}) })} className="size-4 accent-[var(--color-accent)]" />
        {unwatchedLabel ?? t('unwatched')}
      </label>
    </>
  );
}

/**
 * A video or audio detail the library's files have, offered only when it has more than one (or
 * one is already chosen), most titles first.
 */
function FacetSelect({ label, any, values, value, name, onChange }: {
  label: string; any: string; values?: Facet[]; value?: string; name: (v: string) => string; onChange: (v: string) => void;
}) {
  const list = [...(values ?? [])];
  if (list.length < 2 && !value) return null;
  if (value && !list.some((x) => x.value === value)) list.push({ value, count: 0 });
  return (
    <Field label={label}>
      <select value={value ?? ''} onChange={(e) => onChange(e.target.value)} className={inputClass}>
        <option value="">{any}</option>
        {list.map((x) => (
          <option key={x.value} value={x.value}>
            {x.count ? `${name(x.value)} (${x.count})` : name(x.value)}
          </option>
        ))}
      </select>
    </Field>
  );
}

/** How many titles a filter matches right now, with the first few posters. */
export function FilterPreview({ filter: f, library, emptyText }: { filter: RowFilter; library?: Library; emptyText: string }) {
  const { t } = useTranslation('filters');
  const movies = library?.kind !== 'shows';
  const preview = useQuery({
    queryKey: ['items', 'filter-preview', f],
    queryFn: () => api.items(rowQuery(f, library, 8)),
    enabled: !!f.library_id,
    placeholderData: (prev) => prev,
  });
  return (
    <div className="rounded-2xl bg-surface-2/60 p-3 ring-1 ring-border">
      <p className="mb-2 text-xs font-medium text-muted">
        {preview.data
          ? preview.data.total === 0
            ? emptyText
            : movies ? t('preview.movies', { count: preview.data.total }) : t('preview.shows', { count: preview.data.total })
          : preview.error ? t('preview.unavailable') : t('preview.loading')}
      </p>
      <ErrorNotice error={preview.error} onRetry={() => void preview.refetch()} />
      <div className="grid grid-cols-8 gap-1.5">
        {preview.data?.items.map((it) => (
          <Artwork key={it.id} src={it.images?.primary} alt={it.title} className="aspect-[2/3] rounded-md" fallbackTitle={it.title} />
        ))}
      </div>
    </div>
  );
}

/** Shared date, runtime and rating filters for browsing and saved rules. */
export function AdvancedFilterFields({ filter: f, onChange }: { filter: RowFilter; onChange: (f: RowFilter) => void }) {
  const { t } = useTranslation('filters');
  const set = (patch: Partial<RowFilter>) => onChange({ ...f, ...patch });
  const lengthOptions = getLengthOptions();
  const length = lengthOptions.findIndex((o) => o.min === f.min_runtime && o.max === f.max_runtime);
  return <>        <Field label={t('added.label')}>
          <select value={f.added_within ?? 0} onChange={(e) => set({ added_within: Number(e.target.value) || undefined })} className={inputClass}>
            {addedOptions.map((d) => (
              <option key={d} value={d}>
                {d === 0 ? t('added.any') : d === 7 ? t('added.week') : d === 365 ? t('added.year') : t('added.lastDays', { count: d })}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t('length.label')}>
          <select
            value={Math.max(0, length)}
            onChange={(e) => {
              const o = lengthOptions[Number(e.target.value)];
              set({ min_runtime: o.min, max_runtime: o.max });
            }}
            className={inputClass}
          >
            {lengthOptions.map((o, i) => (
              <option key={o.label} value={i}>
                {o.label}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t('rating.label')}>
          <select value={f.min_rating ?? 0} onChange={(e) => set({ min_rating: Number(e.target.value) || undefined })} className={inputClass}>
            {ratingOptions.map((r) => (
              <option key={r} value={r}>
                {r === 0 ? t('rating.any') : t('rating.atLeast', { rating: r })}
              </option>
            ))}
          </select>
        </Field>
</>;
}
