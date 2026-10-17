import { useState } from 'react';
import { Reorder, useDragControls } from 'motion/react';
import { CalendarHeart, ChevronLeft, Eye, EyeOff, Filter, GripVertical, Pencil, Plus, Trash2 } from 'lucide-react';
import type { Library, RowFilter } from '@/lib/api';
import { defaultRowIds, resolveRows, seasons, toSaved, usePrefs, useUpdatePrefs, visibleRows, type ResolvedRow } from '@/lib/home';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { resolutions, sorts } from '@/pages/Library';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import { Field, inputClass } from '@/components/ui/field';
import { FilterFields, FilterPreview } from '@/components/FilterFields';

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
  const prefs = usePrefs();
  const [split, setSplit] = useState(() => !!prefs.data?.split_up_next);
  const save = useUpdatePrefs();
  // Reordering only sees the rows on show; Up Next keeps its place after Continue Watching.
  const reorder = (shown: ResolvedRow[]) => {
    if (split) return setRows(shown);
    const upNext = rows.find((r) => r.type === 'next-up');
    const next = [...shown];
    if (upNext) next.splice(next.findIndex((r) => r.type === 'continue') + 1, 0, upNext);
    setRows(next);
  };

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
              save.mutate({ home: isDefault ? [] : saved, split_up_next: split }, { onSuccess: onDone });
            }}
          >
            Done
          </Button>
        </>
      }
    >
      <label className="mb-4 flex cursor-pointer items-center gap-4 rounded-2xl bg-surface-2/40 px-4 py-3 ring-1 ring-border">
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-medium">Show Up Next separately</span>
          <span className="block text-xs text-subtle">
            {split
              ? 'Next episodes have their own row, apart from what you\'re part-way through.'
              : 'Next episodes of your shows are in Continue Watching, as in Plex.'}
          </span>
        </span>
        <Switch
          checked={split}
          onCheckedChange={(on) => {
            setSplit(on);
            // Splitting it out is pointless if the row is hidden, so show it.
            if (on) setRows((rs) => rs.map((r) => (r.type === 'next-up' ? { ...r, hidden: false } : r)));
          }}
        />
      </label>
      <Reorder.Group axis="y" values={visibleRows(rows, split)} onReorder={reorder} className="space-y-1.5">
        {visibleRows(rows, split).map((r) => (
          <RowItem
            key={r.id}
            row={r}
            merged={!split}
            onToggle={() => setRows((rs) => rs.map((x) => (x.id === r.id ? { ...x, hidden: !x.hidden } : x)))}
            onEdit={r.type === 'custom' ? () => setEditing(r) : undefined}
            onDelete={r.type === 'custom' ? () => setRows((rs) => rs.filter((x) => x.id !== r.id)) : undefined}
          />
        ))}
      </Reorder.Group>
    </DialogContent>
  );
}

function describe(r: ResolvedRow, merged: boolean): string {
  switch (r.type) {
    case 'continue':
      return merged ? "What you're part-way through, and the next episode of each show" : "What you're part-way through";
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

function RowItem({
  row,
  merged,
  onToggle,
  onEdit,
  onDelete,
}: {
  row: ResolvedRow;
  merged: boolean;
  onToggle: () => void;
  onEdit?: () => void;
  onDelete?: () => void;
}) {
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
        <p className="truncate text-xs text-subtle">{describe(row, merged)}</p>
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
        <FilterFields filter={f} onChange={setF} libraries={libraries} />
        <FilterPreview filter={f} library={library} emptyText="Nothing matches yet. The row stays hidden until something does." />
      </div>
    </DialogContent>
  );
}

