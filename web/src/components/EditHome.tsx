import { useState } from 'react';
import { Reorder, useDragControls } from 'motion/react';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { CalendarHeart, ChevronLeft, Eye, EyeOff, Filter, GripVertical, Layers, ListVideo, Pencil, Plus, RectangleHorizontal, RectangleVertical, Trash2 } from 'lucide-react';
import { api, type Library, type Prefs, type RowFilter } from '@/lib/api';
import { useLibraries } from '@/components/Sidebar';
import { defaultRowIds, hasEpisodes, isDefaultLook, resolveRows, seasons, toSaved, usePrefs, useUpdatePrefs, visibleRows, type ResolvedRow, type RowLook } from '@/lib/home';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { resolutions, sorts } from '@/pages/Library';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
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
  const prefs = usePrefs();
  const update = useUpdatePrefs();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && (
        <Editor
          initial={rows}
          libraries={libraries}
          initialSplit={!!prefs.data?.split_up_next}
          onSave={(patch) => update.mutateAsync(patch)}
          onDone={() => onOpenChange(false)}
        />
      )}
    </Dialog>
  );
}

/** Whose home is being arranged by the admin: another profile's, or the default for profiles that haven't arranged their own. */
export type HomeTarget = { kind: 'profile'; id: string; name: string } | { kind: 'default' };

/** The admin arranging another profile's home page, or the default home. */
export function EditOtherHomeDialog({ target, open, onOpenChange }: { target: HomeTarget | null; open: boolean; onOpenChange: (open: boolean) => void }) {
  const qc = useQueryClient();
  const { data: libraries } = useLibraries();
  const key = target?.kind === 'profile' ? ['prefs', 'profile', target.id] : ['prefs', 'default'];
  const prefs = useQuery({
    queryKey: key,
    enabled: open && !!target,
    queryFn: async (): Promise<Pick<Prefs, 'home' | 'split_up_next' | 'home_is_default'>> =>
      target?.kind === 'profile' ? api.profilePrefs(target.id) : { ...(await api.defaultHome()), split_up_next: false },
  });
  const ready = open && target && prefs.data && libraries;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {ready && (
        <Editor
          key={key.join(':')}
          initial={resolveRows(prefs.data!.home, libraries!)}
          libraries={libraries!}
          initialSplit={!!prefs.data!.split_up_next}
          showSplit={target!.kind === 'profile'}
          title={target!.kind === 'profile' ? `${possessive(target!.name)} Home` : 'Default Home'}
          description={
            target!.kind === 'profile'
              ? `Arrange ${possessive(target!.name)} home page. They can still change it themselves.`
              : "The home page for profiles that haven't arranged their own, including new ones."
          }
          onSave={async (patch) => {
            if (target!.kind === 'profile') await api.updateProfilePrefs(target!.id, patch);
            else await api.setDefaultHome(patch.home ?? []);
            qc.invalidateQueries({ queryKey: ['prefs'] });
          }}
          onDone={() => onOpenChange(false)}
        />
      )}
    </Dialog>
  );
}

/** "Sam's", "Kids'". */
function possessive(name: string) {
  return name.endsWith('s') ? `${name}'` : `${name}'s`;
}

function Editor({
  initial,
  libraries,
  initialSplit,
  showSplit = true,
  title = 'Edit Home',
  description = "Drag rows to reorder them, hide the ones you don't want, or add your own. This is just for your profile.",
  onSave,
  onDone,
}: {
  initial: ResolvedRow[];
  libraries: Library[];
  initialSplit: boolean;
  showSplit?: boolean;
  title?: string;
  description?: string;
  onSave: (patch: Partial<Prefs>) => Promise<unknown>;
  onDone: () => void;
}) {
  const [rows, setRows] = useState(initial);
  const [editing, setEditing] = useState<ResolvedRow | 'new' | null>(null);
  const [split, setSplit] = useState(initialSplit);
  const save = useMutation({ mutationFn: onSave, onSuccess: onDone });
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
        onPick={(kind, ref, title) => {
          const id = `${kind}:${ref}`;
          setRows((rs) => {
            if (rs.some((r) => r.id === id)) return rs;
            const at = rs.findIndex((r) => r.type !== 'continue' && r.type !== 'next-up');
            const next = [...rs];
            next.splice(at < 0 ? rs.length : at, 0, { id, hidden: false, type: kind, title, ref, style: 'poster', showsOnly: false, posters: 'series' });
            return next;
          });
          setEditing(null);
        }}
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
              next.splice(at < 0 ? rs.length : at, 0, { id, hidden: false, type: 'custom', title, filter, library, style: 'poster', showsOnly: false, posters: 'series' });
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
      title={title}
      description={description}
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
                rows.every((r) => !r.hidden && r.type !== 'custom' && isDefaultLook(r)) && rows.map((r) => r.id).join() === defaultRowIds(libraries).join();
              save.mutate(showSplit ? { home: isDefault ? [] : saved, split_up_next: split } : { home: isDefault ? [] : saved });
            }}
          >
            Done
          </Button>
        </>
      }
    >
      <label className={cn('mb-4 flex cursor-pointer items-center gap-4 rounded-2xl bg-surface-2/40 px-4 py-3 ring-1 ring-border', !showSplit && 'hidden')}>
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
            onChange={(patch) => setRows((rs) => rs.map((x) => (x.id === r.id ? ({ ...x, ...patch } as ResolvedRow) : x)))}
            onEdit={r.type === 'custom' ? () => setEditing(r) : undefined}
            onDelete={r.type === 'custom' || r.type === 'collection' || r.type === 'playlist' ? () => setRows((rs) => rs.filter((x) => x.id !== r.id)) : undefined}
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
      return r.library.kind === 'shows' && !r.showsOnly ? `Newest in ${r.library.name}, new episodes grouped by season` : `Newest in ${r.library.name}`;
    case 'seasonal':
      return seasonsSummary;
    case 'custom':
      return filterSummary(r.filter, r.library);
    case 'watchlist':
      return "Movies and shows you've saved for later";
    case 'collection':
      return 'A collection';
    case 'playlist':
      return 'Your playlist, in its order';
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
  if (f.language) parts.push(`in ${fmt.language(f.language)}`);
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

const lookLabels = { series: 'Show posters', season: 'Season posters', thumb: 'Thumbnails', poster: 'Posters' };

/** How a row's cards look: posters (the show's or the season's, for rows with episodes) or thumbnails. */
function LookMenu({ row, onChange }: { row: ResolvedRow; onChange: (patch: Partial<RowLook>) => void }) {
  const episodes = hasEpisodes(row);
  const value = row.style === 'thumb' ? 'thumb' : episodes ? row.posters : 'poster';
  const options = episodes ? (['series', 'season', 'thumb'] as const) : (['poster', 'thumb'] as const);
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={`Card style: ${lookLabels[value]}`} title={`Card style: ${lookLabels[value]}`}>
          {value === 'thumb' ? <RectangleHorizontal /> : <RectangleVertical />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuLabel>Show as</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={value}
          onValueChange={(v) =>
            onChange(v === 'thumb' ? { style: 'thumb' } : v === 'poster' ? { style: 'poster' } : { style: 'poster', posters: v as RowLook['posters'] })
          }
        >
          {options.map((o) => (
            <DropdownMenuRadioItem key={o} value={o}>
              {lookLabels[o]}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function RowItem({
  row,
  merged,
  onToggle,
  onChange,
  onEdit,
  onDelete,
}: {
  row: ResolvedRow;
  merged: boolean;
  onToggle: () => void;
  onChange: (patch: Partial<RowLook>) => void;
  onEdit?: () => void;
  onDelete?: () => void;
}) {
  const drag = useDragControls();
  const icon =
    row.type === 'custom' ? (
      <Filter className="size-4" />
    ) : row.type === 'seasonal' ? (
      <CalendarHeart className="size-4" />
    ) : row.type === 'collection' ? (
      <Layers className="size-4" />
    ) : row.type === 'playlist' ? (
      <ListVideo className="size-4" />
    ) : null;
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
      {row.type === 'recent' && row.library.kind === 'shows' && (
        <Button
          variant="ghost"
          size="icon"
          aria-pressed={!row.showsOnly}
          aria-label={row.showsOnly ? 'Group new episodes by season' : 'Show one card per show'}
          title={row.showsOnly ? 'One card per show. Click to group new episodes by season, with season posters.' : 'New episodes grouped by season, with season posters. Click for one card per show.'}
          onClick={() => onChange({ showsOnly: !row.showsOnly })}
          className={cn(!row.showsOnly && 'text-accent')}
        >
          <Layers />
        </Button>
      )}
      <LookMenu row={row} onChange={onChange} />
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
  onPick,
}: {
  libraries: Library[];
  initial?: Extract<ResolvedRow, { type: 'custom' }>;
  onCancel: () => void;
  onSave: (title: string, filter: RowFilter) => void;
  /** A collection or playlist was chosen instead of filters. */
  onPick: (kind: 'collection' | 'playlist', ref: string, title: string) => void;
}) {
  // What the new row shows: titles matching filters, a collection, or a playlist.
  const [source, setSource] = useState<'filter' | 'collection' | 'playlist'>('filter');
  const [picked, setPicked] = useState('');
  const collections = useQueries({
    queries: libraries.map((l) => ({ queryKey: ['collections', l.id], queryFn: () => api.collections(l.id), enabled: source === 'collection' })),
  });
  const playlists = useQuery({ queryKey: ['playlists'], queryFn: api.playlists, enabled: source === 'playlist' });
  const options =
    source === 'collection'
      ? collections.flatMap((q, i) => (q.data ?? []).map((c) => ({ id: c.id, title: c.title, group: libraries[i].name })))
      : (playlists.data ?? []).map((p) => ({ id: p.id, title: p.title, group: '' }));
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
          {source === 'filter' ? (
            <Button variant="primary" disabled={!title.trim() || !f.library_id} onClick={() => onSave(title.trim(), f)}>
              {initial ? 'Save Row' : 'Add Row'}
            </Button>
          ) : (
            <Button
              variant="primary"
              disabled={!picked}
              onClick={() => onPick(source, picked, options.find((o) => o.id === picked)?.title ?? '')}
            >
              Add Row
            </Button>
          )}
        </>
      }
    >
      <div className="space-y-4">
        {!initial && (
          <Field label="Show">
            <select
              value={source}
              onChange={(e) => {
                setSource(e.target.value as typeof source);
                setPicked('');
              }}
              className={inputClass}
            >
              <option value="filter">Titles that match filters</option>
              <option value="collection">A collection</option>
              <option value="playlist">One of your playlists</option>
            </select>
          </Field>
        )}
        {source !== 'filter' ? (
          <Field label={source === 'collection' ? 'Collection' : 'Playlist'}>
            <select value={picked} onChange={(e) => setPicked(e.target.value)} className={inputClass}>
              <option value="">{options.length ? 'Choose…' : source === 'collection' ? 'No collections yet' : 'No playlists yet'}</option>
              {options.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.group ? `${o.title} (${o.group})` : o.title}
                </option>
              ))}
            </select>
          </Field>
        ) : (
          <>
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
          </>
        )}
      </div>
    </DialogContent>
  );
}

