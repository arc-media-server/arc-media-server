import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Reorder, useDragControls } from 'motion/react';
import { CalendarHeart, ChevronLeft, Eye, EyeOff, Filter, GripVertical, Pencil, Plus, Trash2 } from 'lucide-react';
import { api, type ItemSort, type Library, type Resolution, type RowFilter } from '@/lib/api';
import { defaultRowIds, resolveRows, rowQuery, seasons, toSaved, useUpdatePrefs, type ResolvedRow } from '@/lib/home';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { resolutions, sorts } from '@/pages/Library';
import { Artwork } from '@/components/Artwork';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Field, inputClass } from '@/components/ui/field';

/** Reorder, hide and add rows on the profile's home page. */
export function EditHomeDialog({
  rows,
  libraries,
  open,
  onOpenChange,
}: {
  rows: ResolvedRow[];
  libraries: Library[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <Editor initial={rows} libraries={libraries} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function Editor({ initial, libraries, onDone }: { initial: ResolvedRow[]; libraries: Library[]; onDone: () => void }) {
  const [rows, setRows] = useState(initial);
  const [editing, setEditing] = useState<ResolvedRow | 'new' | null>(null);
  const save = useUpdatePrefs();

  if (editing) {
    const existing = editing === 'new' ? undefined : (editing as Extract<ResolvedRow, { type: 'custom' }>);
    return (
      <RowEditor
        libraries={libraries}
        initial={existing}
        onCancel={() => setEditing(null)}
        onSave={(title, filter) => {
          const library = libraries.find((l) => l.id === filter.library_id);
          if (existing) {
            setRows((rs) => rs.map((r) => (r.id === existing.id ? { ...existing, title, filter, library } : r)));
          } else {
            const id = `custom:${Date.now().toString(36)}`;
            // New rows go after carrying on watching, where they're seen.
            setRows((rs) => {
              const at = rs.findIndex((r) => r.type !== 'continue' && r.type !== 'next-up');
              const next = [...rs];
              next.splice(at < 0 ? rs.length : at, 0, { id, hidden: false, type: 'custom', title, filter, library });
              return next;
            });
          }
          setEditing(null);
        }}
      />
    );
  }

  return (
    <DialogContent
      title="Edit Home"
      description="Drag rows to reorder them, hide the ones you don't want, or add your own. This is just for your profile."
      className="w-[min(600px,calc(100vw-32px))]"
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <Button variant="ghost" className="mr-auto" onClick={() => setRows(resolveRows([], libraries).concat(rows.filter((r) => r.type === 'custom')))}>
            Reset Order
          </Button>
          <Button variant="secondary" onClick={() => setEditing('new')}>
            <Plus /> Add Row
          </Button>
          <Button
            variant="primary"
            disabled={save.isPending}
            onClick={() => {
              // An untouched default layout is saved as "default", so later changes to it apply.
              const saved = toSaved(rows);
              const isDefault =
                rows.every((r) => !r.hidden && r.type !== 'custom') && rows.map((r) => r.id).join() === defaultRowIds(libraries).join();
              save.mutate({ home: isDefault ? [] : saved }, { onSuccess: onDone });
            }}
          >
            Done
          </Button>
        </>
      }
    >
      <Reorder.Group axis="y" values={rows} onReorder={setRows} className="space-y-1.5">
        {rows.map((r) => (
          <RowItem
            key={r.id}
            row={r}
            onToggle={() => setRows((rs) => rs.map((x) => (x.id === r.id ? { ...x, hidden: !x.hidden } : x)))}
            onEdit={r.type === 'custom' ? () => setEditing(r) : undefined}
            onDelete={r.type === 'custom' ? () => setRows((rs) => rs.filter((x) => x.id !== r.id)) : undefined}
          />
        ))}
      </Reorder.Group>
    </DialogContent>
  );
}

function describe(r: ResolvedRow): string {
  switch (r.type) {
    case 'continue':
      return "What you're part-way through";
    case 'next-up':
      return 'The next episode of each show you watch';
    case 'recent':
      return `Newest in ${r.library.name}`;
    case 'seasonal':
      return seasonsSummary;
    case 'custom':
      return filterSummary(r.filter, r.library);
  }
}

const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
// e.g. "Romance for Valentine's Day (1–14 Feb) · Horror for Halloween (Oct) · …"
const seasonsSummary = seasons
  .map((s) => {
    const whole = s.from[1] === 1 && s.from[0] === s.to[0] && s.to[1] >= 28;
    const when = whole ? months[s.from[0] - 1] : `${s.from[1]}–${s.to[1]} ${months[s.to[0] - 1]}`;
    return `${s.title} (${when})`;
  })
  .join(' · ');

function filterSummary(f: RowFilter, library?: Library) {
  const parts = [library?.name ?? 'Library'];
  if (f.unwatched) parts.push('unwatched');
  if (f.genre) parts.push(f.genre);
  if (f.decade) parts.push(`${f.decade}s`);
  if (f.resolution) parts.push(resolutions[f.resolution]);
  if (f.added_within) parts.push(`added in the last ${fmt.plural(f.added_within, 'day')}`);
  if (f.max_runtime) parts.push(`under ${runtimeLabel(f.max_runtime)}`);
  if (f.min_runtime) parts.push(`over ${runtimeLabel(f.min_runtime)}`);
  if (f.min_rating) parts.push(`rated ${f.min_rating}+`);
  if (f.sort) parts.push(f.sort === 'shuffle' ? 'shuffled' : `by ${sorts[f.sort].toLowerCase()}`);
  return parts.join(' · ');
}

function runtimeLabel(min: number) {
  return min % 60 === 0 ? fmt.plural(min / 60, 'hour') : `${min} min`;
}

function RowItem({ row, onToggle, onEdit, onDelete }: { row: ResolvedRow; onToggle: () => void; onEdit?: () => void; onDelete?: () => void }) {
  const drag = useDragControls();
  const icon =
    row.type === 'custom' ? <Filter className="size-4" /> : row.type === 'seasonal' ? <CalendarHeart className="size-4" /> : null;
  return (
    <Reorder.Item
      value={row}
      dragListener={false}
      dragControls={drag}
      className="flex items-center gap-2 rounded-2xl bg-surface-2/70 py-2 pl-1.5 pr-2 ring-1 ring-border"
      whileDrag={{ scale: 1.02, boxShadow: '0 12px 30px rgba(0,0,0,0.35)' }}
    >
      <button
        type="button"
        aria-label={`Drag ${row.title}`}
        onPointerDown={(e) => drag.start(e)}
        className="cursor-grab touch-none rounded-lg p-1.5 text-subtle hover:text-foreground active:cursor-grabbing"
      >
        <GripVertical className="size-4" />
      </button>
      <div className={cn('min-w-0 flex-1', row.hidden && 'opacity-45')}>
        <p className="flex items-center gap-1.5 truncate text-sm font-medium">
          {icon && <span className="text-accent">{icon}</span>}
          {row.type === 'seasonal' ? 'Seasonal Picks' : row.title}
        </p>
        <p className="truncate text-xs text-subtle">{describe(row)}</p>
      </div>
      {onEdit && (
        <Button variant="ghost" size="icon" aria-label={`Edit ${row.title}`} onClick={onEdit}>
          <Pencil />
        </Button>
      )}
      {onDelete && (
        <Button variant="ghost" size="icon" aria-label={`Remove ${row.title}`} onClick={onDelete}>
          <Trash2 />
        </Button>
      )}
      <Button variant="ghost" size="icon" aria-label={row.hidden ? `Show ${row.title}` : `Hide ${row.title}`} onClick={onToggle}>
        {row.hidden ? <EyeOff className="text-subtle" /> : <Eye />}
      </Button>
    </Reorder.Item>
  );
}

const addedOptions = [0, 7, 30, 90, 365];
const lengthOptions: { label: string; min?: number; max?: number }[] = [
  { label: 'Any length' },
  { label: 'Under 90 minutes', max: 90 },
  { label: 'Under 2 hours', max: 120 },
  { label: 'Over 2 hours', min: 120 },
  { label: 'Over 2½ hours', min: 150 },
];
const ratingOptions = [0, 6, 7, 8];

function RowEditor({
  libraries,
  initial,
  onCancel,
  onSave,
}: {
  libraries: Library[];
  initial?: Extract<ResolvedRow, { type: 'custom' }>;
  onCancel: () => void;
  onSave: (title: string, filter: RowFilter) => void;
}) {
  const [title, setTitle] = useState(initial?.title ?? '');
  const [f, setF] = useState<RowFilter>(initial?.filter ?? { library_id: libraries[0]?.id ?? 0, sort: 'added' });
  const library = libraries.find((l) => l.id === f.library_id);
  const movies = library?.kind !== 'shows';
  const facets = useQuery({ queryKey: ['facets', f.library_id], queryFn: () => api.facets(f.library_id), enabled: !!f.library_id });
  const preview = useQuery({
    queryKey: ['items', 'row-preview', f],
    queryFn: () => api.items(rowQuery(f, library, 8)),
    enabled: !!f.library_id,
    placeholderData: (prev) => prev,
  });
  const set = (patch: Partial<RowFilter>) => setF((cur) => ({ ...cur, ...patch }));
  const length = lengthOptions.findIndex((o) => o.min === f.min_runtime && o.max === f.max_runtime);

  return (
    <DialogContent
      title={initial ? 'Edit Row' : 'Add a Row'}
      description="Choose what the row shows. It updates by itself as your library changes."
      className="w-[min(600px,calc(100vw-32px))]"
      footer={
        <>
          <Button variant="ghost" className="mr-auto" onClick={onCancel}>
            <ChevronLeft /> Back
          </Button>
          <Button variant="primary" disabled={!title.trim() || !f.library_id} onClick={() => onSave(title.trim(), f)}>
            {initial ? 'Save Row' : 'Add Row'}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label="Name">
          <input
            autoFocus
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={60}
            placeholder="e.g. Unwatched 90s Action"
            className={inputClass}
          />
        </Field>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Library">
            <select
              value={f.library_id}
              onChange={(e) => setF({ library_id: Number(e.target.value), sort: f.sort, unwatched: f.unwatched })}
              className={inputClass}
            >
              {libraries.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </select>
          </Field>
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
          Only things I haven't watched
        </label>

        <div className="rounded-2xl bg-surface-2/60 p-3 ring-1 ring-border">
          <p className="mb-2 text-xs font-medium text-muted">
            {preview.data ? (preview.data.total === 0 ? 'Nothing matches yet. The row stays hidden until something does.' : `${fmt.plural(preview.data.total, movies ? 'movie' : 'show')} ${preview.data.total === 1 ? 'matches' : 'match'}`) : 'Looking…'}
          </p>
          <div className="grid grid-cols-8 gap-1.5">
            {preview.data?.items.map((it) => (
              <Artwork key={it.id} src={it.images?.primary} alt={it.title} className="aspect-[2/3] rounded-md" fallbackTitle={it.title} />
            ))}
          </div>
        </div>
      </div>
    </DialogContent>
  );
}

