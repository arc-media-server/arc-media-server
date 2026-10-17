import { useQuery } from '@tanstack/react-query';
import { api, type ItemSort, type Library, type Resolution, type RowFilter } from '@/lib/api';
import { rowQuery } from '@/lib/home';
import * as fmt from '@/lib/format';
import { resolutions, sorts } from '@/pages/Library';
import { Artwork } from '@/components/Artwork';
import { Field, inputClass } from '@/components/ui/field';

const addedOptions = [0, 7, 30, 90, 365];
const lengthOptions: { label: string; min?: number; max?: number }[] = [
  { label: 'Any length' },
  { label: 'Under 90 minutes', max: 90 },
  { label: 'Under 2 hours', max: 120 },
  { label: 'Over 2 hours', min: 120 },
  { label: 'Over 2½ hours', min: 150 },
];
const ratingOptions = [0, 6, 7, 8];

/**
 * The filters a custom home row or smart collection is made of. With libraries, the library can
 * be chosen too.
 */
export function FilterFields({
  filter: f,
  onChange,
  libraries,
  unwatchedLabel = "Only things I haven't watched",
}: {
  filter: RowFilter;
  onChange: (f: RowFilter) => void;
  libraries?: Library[];
  unwatchedLabel?: string;
}) {
  const facets = useQuery({ queryKey: ['facets', f.library_id], queryFn: () => api.facets(f.library_id), enabled: !!f.library_id });
  const library = libraries?.find((l) => l.id === f.library_id);
  const movies = library ? library.kind !== 'shows' : true;
  const set = (patch: Partial<RowFilter>) => onChange({ ...f, ...patch });
  const length = lengthOptions.findIndex((o) => o.min === f.min_runtime && o.max === f.max_runtime);

  return (
    <>
      <div className="grid gap-3 sm:grid-cols-2">
        {libraries && (
          <Field label="Library">
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
        <Field label="Order">
          <select value={f.sort ?? 'title'} onChange={(e) => set({ sort: e.target.value as ItemSort })} className={inputClass}>
            {Object.entries(sorts).map(([k, label]) => (
              <option key={k} value={k}>
                {label}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Genre">
          <select value={f.genre ?? ''} onChange={(e) => set({ genre: e.target.value || undefined })} className={inputClass}>
            <option value="">Any genre</option>
            {facets.data?.genres.map((g) => (
              <option key={g.value} value={g.value}>
                {g.value}
              </option>
            ))}
          </select>
        </Field>
        {(facets.data?.labels.length ?? 0) > 0 && (
          <Field label="Label">
            <select value={f.label ?? ''} onChange={(e) => set({ label: e.target.value || undefined })} className={inputClass}>
              <option value="">Any label</option>
              {facets.data?.labels.map((l) => (
                <option key={l.value} value={l.value}>
                  {l.value}
                </option>
              ))}
            </select>
          </Field>
        )}
        {(facets.data?.languages.length ?? 0) > 1 && (
          <Field label="Original language">
            <select value={f.language ?? ''} onChange={(e) => set({ language: e.target.value || undefined })} className={inputClass}>
              <option value="">Any language</option>
              {facets.data?.languages.map((l) => (
                <option key={l.value} value={l.value}>
                  {fmt.language(l.value)}
                </option>
              ))}
            </select>
          </Field>
        )}
        <Field label="Decade">
          <select value={f.decade ?? ''} onChange={(e) => set({ decade: Number(e.target.value) || undefined })} className={inputClass}>
            <option value="">Any decade</option>
            {facets.data?.decades.map((d) => (
              <option key={d.value} value={d.value}>
                {d.value}s
              </option>
            ))}
          </select>
        </Field>
        <Field label="Added">
          <select value={f.added_within ?? 0} onChange={(e) => set({ added_within: Number(e.target.value) || undefined })} className={inputClass}>
            {addedOptions.map((d) => (
              <option key={d} value={d}>
                {d === 0 ? 'Any time' : d === 7 ? 'This week' : d === 365 ? 'This year' : `In the last ${d} days`}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Length">
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
        <Field label="Rating">
          <select value={f.min_rating ?? 0} onChange={(e) => set({ min_rating: Number(e.target.value) || undefined })} className={inputClass}>
            {ratingOptions.map((r) => (
              <option key={r} value={r}>
                {r === 0 ? 'Any rating' : `★ ${r} or more`}
              </option>
            ))}
          </select>
        </Field>
        {movies && (
          <Field label="Quality">
            <select value={f.resolution ?? ''} onChange={(e) => set({ resolution: (e.target.value as Resolution) || undefined })} className={inputClass}>
              <option value="">Any quality</option>
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
        <input type="checkbox" checked={!!f.unwatched} onChange={(e) => set({ unwatched: e.target.checked || undefined })} className="size-4 accent-[var(--color-accent)]" />
        {unwatchedLabel}
      </label>
    </>
  );
}

/** How many titles a filter matches right now, with the first few posters. */
export function FilterPreview({ filter: f, library, emptyText }: { filter: RowFilter; library?: Library; emptyText: string }) {
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
            : `${fmt.plural(preview.data.total, movies ? 'movie' : 'show')} ${preview.data.total === 1 ? 'matches' : 'match'}`
          : 'Looking…'}
      </p>
      <div className="grid grid-cols-8 gap-1.5">
        {preview.data?.items.map((it) => (
          <Artwork key={it.id} src={it.images?.primary} alt={it.title} className="aspect-[2/3] rounded-md" fallbackTitle={it.title} />
        ))}
      </div>
    </div>
  );
}
