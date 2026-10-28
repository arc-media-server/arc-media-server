import { sortSummary } from '@/lib/filter-labels';
import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { Reorder, useDragControls } from 'motion/react';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowUp, ArrowDown, CalendarHeart, ChevronLeft, Eye, EyeOff, Filter, GripVertical, Layers, ListVideo, Pencil, Plus, RectangleHorizontal, RectangleVertical, Square, Trash2 } from 'lucide-react';
import { api, type Library, type Prefs, type RowFilter } from '@/lib/api';
import { useLibraries } from '@/components/Sidebar';
import { defaultRowIds, hasEpisodes, isDefaultLook, resolveRows, rowTitle, seasons, toSaved, usePrefs, useUpdatePrefs, visibleRows, type ResolvedRow, type RowLook } from '@/lib/home';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { resolutions } from '@/pages/Library';
import { ErrorNotice } from './Page';
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
      {open && prefs.data && (
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
  const { t } = useTranslation('home');
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
          title={target!.kind === 'profile' ? target!.name.endsWith('s') ? t('editor.otherTitleEndsS', { name: target!.name }) : t('editor.otherTitle', { name: target!.name }) : t('editor.defaultTitle')}
          description={
            target!.kind === 'profile'
              ? target!.name.endsWith('s') ? t('editor.otherDescriptionEndsS', { name: target!.name }) : t('editor.otherDescription', { name: target!.name })
              : t('editor.defaultDescription')
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


function Editor({
  initial,
  libraries,
  initialSplit,
  showSplit = true,
  title,
  description,
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
  const { t } = useTranslation('home');
  const [rows, setRows] = useState(initial);
  const [editing, setEditing] = useState<ResolvedRow | 'new' | null>(null);
  const [split, setSplit] = useState(initialSplit);
  const [announcement, announce] = useState('');
  const save = useMutation({ mutationFn: onSave, onSuccess: onDone });
  // Reordering only sees the rows on show; Up Next keeps its place after Continue Watching.
  const reorder = (shown: ResolvedRow[]) => {
    if (split) return setRows(shown);
    const upNext = rows.find((r) => r.type === 'next-up');
    const next = [...shown];
    if (upNext) next.splice(next.findIndex((r) => r.type === 'continue') + 1, 0, upNext);
    setRows(next);
  };

  const moveRow = (id: string, delta: number) => {
    const shown = visibleRows(rows, split);
    const from = shown.findIndex(r => r.id === id), to = from + delta;
    if (from < 0 || to < 0 || to >= shown.length) return;
    [shown[from], shown[to]] = [shown[to], shown[from]];
    reorder(shown);
    announce(t('editor.position', { title: rowTitle(shown[to]), position: to + 1, total: shown.length }));
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
      title={title ?? t('editor.title')}
      description={description ?? t('editor.description')}
      className="w-[min(600px,calc(100vw-32px))]"
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <Button variant="ghost" className="mr-auto" onClick={() => setRows(resolveRows([], libraries).map(base => rows.find(r => r.id === base.id) ?? base).concat(rows.filter(r => r.type === 'custom' || r.type === 'collection' || r.type === 'playlist')))}>{t('editor.resetOrder')}</Button>
          <Button variant="secondary" onClick={() => setEditing('new')}>
            <Plus />{' '}{t('editor.addRow')}</Button>
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
          >{t('common:actions.done')}</Button>
        </>
      }
    >
      <label className={cn('mb-4 flex cursor-pointer items-center gap-4 rounded-2xl bg-surface-2/40 px-4 py-3 ring-1 ring-border', !showSplit && 'hidden')}>
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-medium">{t('editor.splitLabel')}</span>
          <span className="block text-xs text-subtle">
            {split
              ? t('editor.splitOn')
              : t('editor.splitOff')}
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
      <p className="sr-only" role="status" aria-live="polite">{announcement}</p>
      <Reorder.Group axis="y" values={visibleRows(rows, split)} onReorder={reorder} className="space-y-1.5">
        {visibleRows(rows, split).map((r, index, shown) => (
          <RowItem
            key={r.id}
            row={r}
            merged={!split}
            onMoveUp={index > 0 ? () => moveRow(r.id, -1) : undefined}
            onMoveDown={index < shown.length - 1 ? () => moveRow(r.id, 1) : undefined}
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
      return merged ? i18n.t('home:rowDescription.continueMerged') : i18n.t('home:rowDescription.continue');
    case 'next-up':
      return i18n.t('home:rowDescription.next');
    case 'recent':
      return r.library.kind === 'shows' && !r.showsOnly ? i18n.t('home:rowDescription.recentSeasons', { name: r.library.name }) : i18n.t('home:rowDescription.recent', { name: r.library.name });
    case 'seasonal':
      return seasonsSummary();
    case 'custom':
      return filterSummary(r.filter, r.library);
    case 'watchlist':
      return i18n.t('home:rowDescription.watchlist');
    case 'collection':
      return i18n.t('home:rowDescription.collection');
    case 'playlist':
      return i18n.t('home:rowDescription.playlist');
  }
}

// e.g. "Romance for Valentine's Day (1–14 Feb) · Horror for Halloween (Oct) · …"
function seasonsSummary() {
  return seasons.map((season) => {
    const whole = season.from[1] === 1 && season.from[0] === season.to[0] && season.to[1] >= 28;
    const when = whole ? fmt.month(season.from[0]) : i18n.t('home:seasonal.range', { start: season.from[1], end: season.to[1], month: fmt.month(season.to[0]) });
    return i18n.t('home:seasonal.summary', { title: season.title, when });
  }).join(' · ');
}

function filterSummary(f: RowFilter, library?: Library) {
  const parts = [library?.name ?? i18n.t('filters:library')];
  if (f.unwatched) parts.push(i18n.t('filters:summary.unwatched'));
  if (f.genre) parts.push(f.genre);
  if (f.label) parts.push(i18n.t('filters:summary.label', { label: f.label }));
  if (f.language) parts.push(i18n.t('filters:summary.language', { language: fmt.language(f.language) }));
  if (f.decade) parts.push(i18n.t('filters:decade.value', { year: f.decade }));
  if (f.resolution) parts.push(resolutions[f.resolution]);
  if (f.added_within) parts.push(i18n.t('filters:summary.addedDays', { count: f.added_within }));
  if (f.max_runtime) parts.push(i18n.t('filters:summary.under', { duration: runtimeLabel(f.max_runtime) }));
  if (f.min_runtime) parts.push(i18n.t('filters:summary.over', { duration: runtimeLabel(f.min_runtime) }));
  if (f.min_rating) parts.push(i18n.t('filters:summary.rated', { rating: f.min_rating }));
  if (f.sort) parts.push(sortSummary(f.sort));
  return parts.join(' · ');
}

function runtimeLabel(min: number) {
  return min % 60 === 0 ? i18n.t('filters:summary.hours', { count: min / 60 }) : i18n.t('filters:summary.minutes', { count: min });
}

function lookLabels() { return { series: i18n.t('home:look.series'), season: i18n.t('home:look.season'), thumb: i18n.t('home:look.thumb'), arc: i18n.t('home:look.arc'), poster: i18n.t('home:look.poster') }; }

/** How a row's cards look: posters (the show's or the season's, for rows with episodes), thumbnails or Arc thumbnails. */
function LookMenu({ row, onChange }: { row: ResolvedRow; onChange: (patch: Partial<RowLook>) => void }) {
  const { t } = useTranslation('home');
  const labels = lookLabels();
  const episodes = hasEpisodes(row);
  const value = row.style !== 'poster' ? row.style : episodes ? row.posters : 'poster';
  const options = episodes ? (['series', 'season', 'thumb', 'arc'] as const) : (['poster', 'thumb', 'arc'] as const);
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t('look.label', { style: labels[value] })} title={t('look.label', { style: labels[value] })}>
          {value === 'thumb' ? <RectangleHorizontal /> : value === 'arc' ? <Square /> : <RectangleVertical />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuLabel>{t('look.showAs')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={value}
          onValueChange={(v) =>
            onChange(v === 'thumb' || v === 'arc' || v === 'poster' ? { style: v } : { style: 'poster', posters: v as RowLook['posters'] })
          }
        >
          {options.map((o) => (
            <DropdownMenuRadioItem key={o} value={o}>
              {labels[o]}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function RowItem({
  onMoveUp, onMoveDown,
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
  onMoveUp?: () => void;
  onMoveDown?: () => void;
  onEdit?: () => void;
  onDelete?: () => void;
}) {
  const { t } = useTranslation('home');
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
        aria-label={t('rowActions.drag', { title: rowTitle(row) })}
        onPointerDown={(e) => drag.start(e)}
        className="cursor-grab touch-none rounded-lg p-1.5 text-subtle hover:text-foreground active:cursor-grabbing"
      >
        <GripVertical className="size-4" />
      </button>
      <div className="flex shrink-0 flex-col">
        <button type="button" aria-label={t('rowActions.moveUp', { title: rowTitle(row) })} disabled={!onMoveUp} onClick={onMoveUp} className="rounded p-1 text-muted hover:bg-surface-3 disabled:opacity-25"><ArrowUp className="size-4" /></button>
        <button type="button" aria-label={t('rowActions.moveDown', { title: rowTitle(row) })} disabled={!onMoveDown} onClick={onMoveDown} className="rounded p-1 text-muted hover:bg-surface-3 disabled:opacity-25"><ArrowDown className="size-4" /></button>
      </div>
      <div className={cn('min-w-0 flex-1', row.hidden && 'opacity-45')}>
        <p className="flex items-center gap-1.5 truncate text-sm font-medium">
          {icon && <span className="text-accent">{icon}</span>}
          {row.type === 'seasonal' ? t('rows.seasonal') : rowTitle(row)}
        </p>
        <p className="truncate text-xs text-subtle">{describe(row, merged)}</p>
      </div>
      {row.type === 'recent' && row.library.kind === 'shows' && (
        <Button
          variant="ghost"
          size="icon"
          aria-pressed={!row.showsOnly}
          aria-label={row.showsOnly ? t('look.groupSeasons') : t('look.groupShows')}
          title={row.showsOnly ? t('look.groupSeasonsHint') : t('look.groupShowsHint')}
          onClick={() => onChange({ showsOnly: !row.showsOnly })}
          className={cn(!row.showsOnly && 'text-accent')}
        >
          <Layers />
        </Button>
      )}
      <LookMenu row={row} onChange={onChange} />
      {onEdit && (
        <Button variant="ghost" size="icon" aria-label={t('rowActions.edit', { title: rowTitle(row) })} onClick={onEdit}>
          <Pencil />
        </Button>
      )}
      {onDelete && (
        <Button variant="ghost" size="icon" aria-label={t('rowActions.remove', { title: rowTitle(row) })} onClick={onDelete}>
          <Trash2 />
        </Button>
      )}
      <Button variant="ghost" size="icon" aria-label={row.hidden ? t('rowActions.show', { title: rowTitle(row) }) : t('rowActions.hide', { title: rowTitle(row) })} onClick={onToggle}>
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
  const { t } = useTranslation('home');
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
      title={initial ? t('rowEditor.edit') : t('rowEditor.add')}
      description={t('rowEditor.description')}
      className="w-[min(600px,calc(100vw-32px))]"
      footer={
        <>
          <Button variant="ghost" className="mr-auto" onClick={onCancel}>
            <ChevronLeft />{' '}{t('common:actions.back')}</Button>
          {source === 'filter' ? (
            <Button variant="primary" disabled={!title.trim() || !f.library_id} onClick={() => onSave(title.trim(), f)}>
              {initial ? t('rowEditor.save') : t('editor.addRow')}
            </Button>
          ) : (
            <Button
              variant="primary"
              disabled={!picked}
              onClick={() => onPick(source, picked, options.find((o) => o.id === picked)?.title ?? '')}
            >{t('editor.addRow')}</Button>
          )}
        </>
      }
    >
      <div className="space-y-4">
        <ErrorNotice error={source === 'playlist' ? playlists.error : source === 'collection' ? collections.find(c => c.error)?.error : null} onRetry={() => { if (source === 'playlist') void playlists.refetch(); else for (const c of collections) if (c.error) void c.refetch(); }} />
        {!initial && (
          <Field label={t('rowEditor.show')}>
            <select
              value={source}
              onChange={(e) => {
                setSource(e.target.value as typeof source);
                setPicked('');
              }}
              className={inputClass}
            >
              <option value="filter">{t('rowEditor.filters')}</option>
              <option value="collection">{t('rowDescription.collection')}</option>
              <option value="playlist">{t('rowEditor.playlist')}</option>
            </select>
          </Field>
        )}
        {source !== 'filter' ? (
          <Field label={source === 'collection' ? t('rowEditor.collection') : t('playlists:kind.manual')}>
            <select value={picked} onChange={(e) => setPicked(e.target.value)} className={inputClass}>
              <option value="">{options.length ? t('rowEditor.choose') : source === 'collection' ? t('rowEditor.noCollections') : t('playlists:empty.heading')}</option>
              {options.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.group ? t('rowEditor.groupedOption', { title: o.title, group: o.group }) : o.title}
                </option>
              ))}
            </select>
          </Field>
        ) : (
          <>
            <Field label={t('common:fields.name')}>
              <input
                autoFocus
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={60}
                placeholder={t('rowEditor.nameHint')}
                className={inputClass}
              />
            </Field>
            <FilterFields filter={f} onChange={setF} libraries={libraries} />
            <FilterPreview filter={f} library={library} emptyText={t('rowEditor.emptyPreview')} />
          </>
        )}
      </div>
    </DialogContent>
  );
}

