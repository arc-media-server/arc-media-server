import { useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, Lock, LockOpen } from 'lucide-react';
import { api, type ArtworkType, type Item, type ItemEdit, type LockableField, type SourceOrdering } from '@/lib/api';
import { applyArtwork, applySeasonArtwork, ArtworkPanel, artworkNames, SeasonsPanel, type ArtworkChange } from '@/components/ChooseArtwork';
import { orderings } from '@/components/LibraryEditor';
import { useLibraries } from '@/components/Sidebar';
import { ChipInput } from '@/components/ChipInput';
import { MarkersPanel, type MarkerEdit } from '@/components/MarkersPanel';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

type TextField = Exclude<LockableField, 'poster' | 'backdrop' | 'logo' | 'genres'>;

interface FieldSpec {
  field: TextField;
  label: string;
  type?: 'text' | 'number' | 'date' | 'textarea';
  placeholder?: string;
  half?: boolean;
}

function fieldsFor(kind: Item['kind']): FieldSpec[] {
  const dateLabel = kind === 'movie' ? 'Release Date' : kind === 'series' ? 'First Aired' : 'Air Date';
  const all: (FieldSpec | false)[] = [
    { field: 'title', label: 'Title' },
    kind !== 'episode' && { field: 'sort_title', label: 'Sort As', placeholder: 'Same as the title' },
    kind !== 'episode' && { field: 'original_title', label: 'Original Title' },
    kind !== 'episode' && { field: 'year', label: 'Year', type: 'number', half: true },
    { field: 'release_date', label: dateLabel, type: 'date', half: true },
    kind !== 'series' && { field: 'runtime_min', label: 'Runtime (minutes)', type: 'number', half: true },
    kind !== 'episode' && { field: 'content_rating', label: 'Age Rating', half: true, placeholder: 'e.g. 12A' },
    kind !== 'episode' && { field: 'studio', label: kind === 'series' ? 'Network' : 'Studio', half: true },
    kind !== 'episode' && { field: 'tagline', label: 'Tagline' },
    { field: 'overview', label: 'Overview', type: 'textarea' },
  ];
  return all.filter((f): f is FieldSpec => !!f);
}

function initialValue(item: Item, field: TextField): string {
  switch (field) {
    case 'year':
      return item.year ? String(item.year) : '';
    case 'runtime_min':
      return item.runtime_min ? String(item.runtime_min) : '';
    case 'sort_title':
      // Only show a sort title the user chose; otherwise it just follows the title.
      return item.locked_fields.includes('sort_title') ? item.sort_title : '';
    default:
      return item[field] ?? '';
  }
}

/** The Edit dialog: correct an item's details (edited fields are locked) and choose its artwork. */
export function EditDetailsDialog({ item, open, onOpenChange }: { item: Item; open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <EditForm item={item} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function EditForm({ item, onDone }: { item: Item; onDone: () => void }) {
  const qc = useQueryClient();
  const specs = fieldsFor(item.kind);
  const [values, setValues] = useState(() => Object.fromEntries(specs.map((s) => [s.field, initialValue(item, s.field)])) as Record<TextField, string>);
  const [locked, setLocked] = useState(() => new Set<LockableField>(item.locked_fields));
  const [genres, setGenres] = useState<string[]>(() => item.genres ?? []);
  const [source, setSource] = useState<SourceOrdering | ''>(() => item.source_ordering ?? '');
  const [detailsFrom, setDetailsFrom] = useState<'' | 'tvdb'>(() => item.details_provider ?? '');
  const library = useLibraries().data?.find((l) => l.id === item.library_id);
  const genresChanged = genres.join('\n') !== (item.genres ?? []).join('\n');
  const [labels, setLabels] = useState<string[]>(() => item.labels ?? []);
  const labelsChanged = labels.join('\n') !== (item.labels ?? []).join('\n');
  const allLabels = useQuery({ queryKey: ['labels'], queryFn: api.labels, enabled: item.kind !== 'episode' });
  const provider = item.kind === 'movie' ? 'TMDB' : 'TMDB and TheTVDB';
  // Movies and shows have artwork to choose; episodes just their details.
  // Episodes and movies also have their skip markers.
  const tabs: ('details' | ArtworkType | 'seasons' | 'markers')[] =
    item.kind === 'episode'
      ? ['details', 'poster', 'markers']
      : item.kind === 'series'
        ? ['details', 'poster', 'backdrop', 'logo', 'seasons']
        : ['details', 'poster', 'backdrop', 'logo', 'markers'];
  const [markerEdit, setMarkerEdit] = useState<{ edit: MarkerEdit | null; valid: boolean }>({ edit: null, valid: true });
  const [tab, setTab] = useState<(typeof tabs)[number]>('details');
  const [art, setArt] = useState<Partial<Record<ArtworkType, ArtworkChange>>>({});
  const [seasonArt, setSeasonArt] = useState<Record<number, ArtworkChange>>({});
  const [saving, setSaving] = useState('Saving details…');

  const save = useMutation({
    mutationFn: async () => {
      const locksChanged = locked.size !== item.locked_fields.length || item.locked_fields.some(f => !locked.has(f));
      const edit: ItemEdit = locksChanged ? { locked_fields: [...locked] } : {};
      for (const s of specs) {
        const v = values[s.field].trim();
        if (v === initialValue(item, s.field).trim()) continue;
        if (s.field === 'year' || s.field === 'runtime_min') edit[s.field] = v ? Number(v) : 0;
        else edit[s.field] = v;
      }
      if (genresChanged) edit.genres = genres;
      if (labelsChanged) edit.labels = labels;
      if (item.kind === 'series' && source !== (item.source_ordering ?? '')) edit.source_ordering = source;
      if (item.kind === 'series' && detailsFrom !== (item.details_provider ?? '')) edit.details_provider = detailsFrom;
      let updated = item;
      if (Object.keys(edit).length) { setSaving('Saving details…'); updated = await api.editItem(item.id, edit); }
      if (markerEdit.edit) {
        setSaving('Saving skip markers…');
        await api.setItemMarkers(item.id, markerEdit.edit);
        qc.invalidateQueries({ queryKey: ['markers', item.id] });
      }
      for (const [type, change] of Object.entries(art) as [ArtworkType, ArtworkChange | undefined][]) {
        if (!change) continue;
        setSaving('Saving ' + artworkNames[type].toLowerCase() + '…');
        updated = await applyArtwork(item.id, type, change);
      }
      for (const [season, change] of Object.entries(seasonArt)) {
        setSaving('Saving season ' + season + ' poster…');
        await applySeasonArtwork(item.id, Number(season), change);
      }
      // Fields just unlocked should go back to the online value straight away.
      if (item.locked_fields.some((f) => !locked.has(f))) {
        setSaving('Refreshing unlocked fields…');
        try {
          updated = await api.refreshItem(item.id);
        } catch {
          // Keep the saved edit; the value updates on the next refresh.
        }
      }
      return updated;
    },
    onSuccess: (updated) => {
      qc.setQueryData(['item', item.id], (old: Item | undefined) => (old ? { ...old, ...updated } : updated));
      qc.invalidateQueries({ queryKey: ['labels'] });
      qc.invalidateQueries({ queryKey: ['facets'] });
      qc.invalidateQueries({ queryKey: ['item', item.id] });
      qc.invalidateQueries({ queryKey: ['seasons'] });
      qc.invalidateQueries({ queryKey: ['items'] });
      qc.invalidateQueries({ queryKey: ['featured'] });
      for (const key of ['artwork', 'continue', 'next-up', 'items']) qc.invalidateQueries({ queryKey: [key] });
      onDone();
    },
  });

  const setValue = (field: TextField, v: string) => {
    setValues((prev) => ({ ...prev, [field]: v }));
    setLocked((prev) => new Set(prev).add(field));
  };
  const toggle = (field: LockableField) =>
    setLocked((prev) => {
      const next = new Set(prev);
      if (next.has(field)) next.delete(field);
      else next.add(field);
      return next;
    });

  return (
    <DialogContent
      title="Edit"
      className={tabs.length > 1 ? 'w-[min(820px,calc(100vw-32px))]' : undefined}
      description={
        tab === 'details' ? <>Anything you change is locked, so refreshing from {provider} won't undo it. Unlock a field to use the online value again.</> : undefined
      }
      footer={
        <>
          {save.isPending && <span role="status" className="mr-auto text-sm text-muted">{saving}</span>}
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost" disabled={save.isPending}>
              Cancel
            </Button>
          </DialogClose>
          <Button variant="primary" onClick={() => save.mutate()} disabled={save.isPending || !values.title?.trim() || !markerEdit.valid}>
            {save.isPending && <Loader2 className="animate-spin" />} Save
          </Button>
        </>
      }
    >
      <fieldset disabled={save.isPending} className="min-w-0">
      {tabs.length > 1 && (
        <div className="mb-5 flex gap-1" role="tablist">
          {tabs.map((t) => (
            <button
              key={t}
              type="button"
              role="tab"
              aria-selected={tab === t}
              onClick={() => setTab(t)}
              className={cn('rounded-full px-3.5 py-1 text-[13px] font-medium transition-colors', tab === t ? 'bg-foreground text-background' : 'text-muted hover:text-foreground')}
            >
              {t === 'details' ? 'Details' : t === 'seasons' ? 'Seasons' : t === 'markers' ? 'Skip Markers' : item.kind === 'episode' ? 'Thumbnail' : artworkNames[t]}
              {(t === 'seasons' ? Object.keys(seasonArt).length > 0 : t === 'markers' ? !!markerEdit.edit : t !== 'details' && art[t]) && <span className="ml-1.5 inline-block size-1.5 rounded-full bg-accent align-middle" aria-label="changed" />}
            </button>
          ))}
        </div>
      )}
      {tab === 'seasons' && (
        <SeasonsPanel
          item={item}
          changes={seasonArt}
          onChange={(n, c) =>
            setSeasonArt((prev) => {
              const next = { ...prev };
              if (c) next[n] = c;
              else delete next[n];
              return next;
            })
          }
        />
      )}
      {tab === 'markers' && <MarkersPanel item={item} onChange={(edit, valid) => setMarkerEdit({ edit, valid })} />}
      {tab !== 'details' && tab !== 'seasons' && tab !== 'markers' && (
        <ArtworkPanel item={item} type={tab} change={art[tab]} onChange={(c) => setArt((prev) => ({ ...prev, [tab]: c }))} />
      )}
      <form
        hidden={tab !== 'details'}
        className="grid grid-cols-2 gap-x-4 gap-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        {specs.map((s) => (
          <Field key={s.field} label={s.label} locked={locked.has(s.field)} onToggle={() => toggle(s.field)} half={s.half}>
            {s.type === 'textarea' ? (
              <textarea
                value={values[s.field]}
                onChange={(e) => setValue(s.field, e.target.value)}
                rows={5}
                className={cn(inputClass, 'h-auto resize-y py-2.5 leading-relaxed')}
              />
            ) : (
              <input
                type={s.type ?? 'text'}
                value={values[s.field]}
                placeholder={s.field === 'sort_title' ? values.title || s.placeholder : s.placeholder}
                min={s.type === 'number' ? 0 : undefined}
                onChange={(e) => setValue(s.field, e.target.value)}
                className={inputClass}
              />
            )}
          </Field>
        ))}
        {item.kind !== 'episode' && (
          <div className="col-span-2">
            <span className="mb-1.5 flex items-center justify-between">
              <span className="text-[11px] font-semibold uppercase tracking-wider text-subtle">Genres</span>
              <LockToggle locked={locked.has('genres')} onToggle={() => toggle('genres')} />
            </span>
            <ChipInput
              value={genres}
              placeholder="e.g. Drama"
              morePlaceholder="Add a genre"
              onChange={(g) => {
                setGenres(g);
                setLocked((prev) => new Set(prev).add('genres'));
              }}
            />
          </div>
        )}
        {item.kind !== 'episode' && (
          <div className="col-span-2">
            <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Labels</span>
            <ChipInput value={labels} onChange={setLabels} placeholder="Your own tags, e.g. Not for kids" morePlaceholder="Add a label" suggestions={allLabels.data} />
            <p className="mt-1.5 text-xs text-subtle">Only you see these. Use them to filter, build smart collections, or hide titles from a profile.</p>
          </div>
        )}
        {item.kind === 'series' && (
          <label className="col-span-2 block">
            <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Details From</span>
            <select value={detailsFrom} onChange={(e) => setDetailsFrom(e.target.value as '' | 'tvdb')} className={inputClass}>
              <option value="">TMDB</option>
              <option value="tvdb">TheTVDB</option>
            </select>
            <span className="mt-1.5 block text-xs text-subtle">
              Where the show&apos;s title, description, artwork and details come from. Only one is used: anything it doesn&apos;t have stays blank.
            </span>
          </label>
        )}
        {item.kind === 'series' && (
          <label className="col-span-2 block">
            <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Episode Numbering</span>
            <select value={source} onChange={(e) => setSource(e.target.value as SourceOrdering | '')} className={inputClass}>
              <option value="">Library default ({orderings.find((o) => o.value === library?.source_ordering)?.label ?? 'TMDB aired order'})</option>
              {orderings.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            <span className="mt-1.5 block text-xs text-subtle">
              How this show's files are numbered. Changing it matches the files to episodes again; watch history is kept. To watch in another
              order, use the episode order menu on the show's page instead.
            </span>
          </label>
        )}
        <button type="submit" hidden />
      </form>
      </fieldset>
    </DialogContent>
  );
}


function Field({ label, locked, onToggle, half, children }: { label: string; locked: boolean; onToggle: () => void; half?: boolean; children: ReactNode }) {
  return (
    <label className={cn('block', half ? 'col-span-2 sm:col-span-1' : 'col-span-2')}>
      <span className="mb-1.5 flex items-center justify-between">
        <span className="text-[11px] font-semibold uppercase tracking-wider text-subtle">{label}</span>
        <LockToggle locked={locked} onToggle={onToggle} />
      </span>
      {children}
    </label>
  );
}

export function LockToggle({ locked, onToggle }: { locked: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={(e) => {
        e.preventDefault();
        onToggle();
      }}
      title={locked ? 'Locked: refreshes keep this' : 'Unlocked: updated from the internet on refresh'}
      aria-pressed={locked}
      className={cn(
        'flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] font-medium transition-colors',
        locked ? 'bg-accent/15 text-accent' : 'text-subtle hover:bg-surface-3 hover:text-muted',
      )}
    >
      {locked ? <Lock className="size-3" /> : <LockOpen className="size-3" />}
      {locked ? 'Locked' : 'Auto'}
    </button>
  );
}

