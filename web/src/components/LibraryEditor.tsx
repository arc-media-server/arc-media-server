import { useEffect, useId, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Check, ChevronDown, Film, FolderPlus, HelpCircle, Loader2, Tv, X, Clapperboard } from 'lucide-react';
import { api, type Library, type LibraryKind, type LibraryPreview, type PreviewItem, type SourceOrdering } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { kindUnit } from '@/lib/kinds';
import { FolderPicker } from '@/components/FolderPicker';
import { gridLabels } from '@/components/Collections';
import { ParsedStep, StepRow } from '@/components/FixMatch';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';
import { Switch } from '@/components/ui/switch';

export const orderings: { value: SourceOrdering; label: string; hint: string }[] = [
  { value: 'tmdb_aired', label: 'TMDB aired order', hint: 'Files numbered the way TMDB lists them' },
  { value: 'tvdb_aired', label: 'TheTVDB aired order', hint: "Files numbered the way TheTVDB lists them (Sonarr's default)" },
  { value: 'tvdb_dvd', label: 'TheTVDB DVD order', hint: 'Files numbered like the DVD releases' },
  { value: 'tvdb_absolute', label: 'TheTVDB absolute order', hint: 'Episodes numbered 1, 2, 3… across seasons (common for anime)' },
];

/** Adds a library for background scanning; a folder preview is optional. */
export function LibraryEditorDialog({ library, open, onOpenChange }: { library?: Library; open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <Editor library={library} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function Editor({ library, onDone }: { library?: Library; onDone: () => void }) {
  const qc = useQueryClient();
  const editing = !!library;
  const fieldId = useId();
  const [kind, setKind] = useState<LibraryKind>(library?.kind ?? 'movies');
  const [name, setName] = useState(library?.name ?? '');
  const [nameTouched, setNameTouched] = useState(editing);
  const [paths, setPaths] = useState<string[]>(library?.paths ?? []);
  const [detailsProvider,setDetailsProvider]=useState<""|"tvdb">(library?.details_provider??"");
  const [ordering, setOrdering] = useState<SourceOrdering>(library?.source_ordering ?? 'tmdb_aired');
  const [trickplay, setTrickplay] = useState(library?.trickplay ?? false);
  // Plex's defaults: intros for TV, credits for TV and movies.
  const [skipIntros, setSkipIntros] = useState(library?.skip_intros ?? true);
  const [skipCredits, setSkipCredits] = useState(library?.skip_credits ?? true);
  const [collectionMin, setCollectionMin] = useState(library?.collection_min ?? 2);
  const [collectionGrid, setCollectionGrid] = useState<Library['collection_grid']>(library?.collection_grid ?? '');
  const [language, setLanguage] = useState(library?.metadata_language ?? '');
  const [picking, setPicking] = useState(!editing);
  const [step, setStep] = useState<'details' | 'preview'>('details');
  const [previewId, setPreviewId] = useState<string | null>(null);

  // Name the library after its kind until the user names it themselves.
  useEffect(() => {
    if (!nameTouched) setName(kind === 'shows' ? 'TV Shows' : kind === 'videos' ? 'Home Videos' : 'Movies');
  }, [kind, nameTouched]);

  // A one-line reminder of the advanced choices while the section is closed.
  const features = [trickplay && 'Scrubbing previews', kind === 'shows' && skipIntros && 'Skip intros', kind !== 'videos' && skipCredits && 'Skip credits'].filter(Boolean);
  const advancedSummary = [
    kind === 'shows' && (detailsProvider === 'tvdb' ? 'TheTVDB' : 'TMDB'),
    kind === 'shows' && orderings.find((o) => o.value === ordering)?.label,
    kind !== 'videos' && metadataLanguages.find(([code]) => code === language)?.[1],
    features.length ? features.join(', ') : 'No player extras',
  ].filter(Boolean).join(' · ');

  const start = useMutation({
    mutationFn: () => api.startPreview({ kind, source_ordering: ordering, paths, metadata_language: language }, library?.id),
    onSuccess: (res) => {
      setPreviewId(res.id);
      setStep('preview');
    },
  });
  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.updateLibrary(library.id, { name, details_provider: kind==='shows'?detailsProvider:"", source_ordering: ordering, paths, trickplay, collection_min: collectionMin, metadata_language: language, skip_intros: skipIntros, skip_credits: skipCredits, collection_grid: collectionGrid })
        : api.createLibrary({ name, kind, details_provider: kind==='shows'?detailsProvider:"", source_ordering: ordering, paths, trickplay, collection_min: collectionMin, metadata_language: language, skip_intros: skipIntros, skip_credits: skipCredits, collection_grid: collectionGrid }),
    onSuccess: () => {
      qc.invalidateQueries();
      onDone();
    },
  });

  const footer =
    step === 'details' ? (
      <>
        {(start.error || save.error) && <p className="mr-auto text-sm text-warning">{(start.error ?? save.error)!.message}</p>}
        <DialogClose asChild>
          <Button variant="ghost">Cancel</Button>
        </DialogClose>
        <Button variant="primary" disabled={!paths.length || !name.trim() || save.isPending || start.isPending} onClick={() => save.mutate()}>
          {save.isPending && <Loader2 className="animate-spin" />} {editing ? 'Save' : 'Add Library'}
        </Button>
      </>
    ) : (
      <>
        {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
        <Button variant="ghost" onClick={() => setStep('details')} disabled={save.isPending}>
          Back
        </Button>
        <Button variant="primary" onClick={() => save.mutate()} disabled={save.isPending}>
          {save.isPending && <Loader2 className="animate-spin" />} Add Library
        </Button>
      </>
    );

  return (
    <DialogContent
      title={editing ? `Edit ${library.name}` : step === 'preview' ? `Preview: ${name}` : 'Add a Library'}
      description={
        step === 'preview'
          ? 'This is what Arc found, before adding anything. Your files are never changed.'
          : editing
            ? 'Folder changes scan the library. Metadata changes refresh known titles.'
            : 'Choose a type and folders. Arc finds titles, matches them and loads artwork in the background.'
      }
      footer={footer}
      className="w-[min(680px,calc(100vw-32px))]"
    >
      {step === 'preview' && previewId ? (
        <PreviewView id={previewId} kind={kind} />
      ) : (
        <div className="space-y-6 pb-2">
          {!editing && (
            <Field label="Type">
              <div className="grid gap-3 sm:grid-cols-3">
                <KindCard icon={<Film />} label="Movies" selected={kind === 'movies'} onSelect={() => setKind('movies')} />
                <KindCard icon={<Tv />} label="TV Shows" selected={kind === 'shows'} onSelect={() => setKind('shows')} />
                <KindCard icon={<Clapperboard />} label="Other Videos" selected={kind === 'videos'} onSelect={() => setKind('videos')} />
              </div>
              {kind === 'videos' && (
                <p className="mt-1.5 text-xs text-subtle">
                  Home videos and anything else that isn't a film or show: named from their files, dated by them, nothing looked up online.
                  Each folder becomes a collection.
                </p>
              )}
            </Field>
          )}
          <Field label="Name" controlId={`${fieldId}-name`}>
            <input
              id={`${fieldId}-name`}
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setNameTouched(true);
              }}
              className={inputClass}
            />
          </Field>
          <Field label="Folders">
            <div className="space-y-2">
              {paths.map((p) => (
                <div key={p} className="flex items-center gap-2 rounded-xl bg-surface-2/80 py-1.5 pl-3 pr-1.5 ring-1 ring-border">
                  <span className="min-w-0 flex-1 truncate font-mono text-xs" title={p}>
                    {p}
                  </span>
                  <Button variant="ghost" size="icon" className="size-7 rounded-lg" aria-label={`Remove ${p}`} onClick={() => setPaths(paths.filter((x) => x !== p))}>
                    <X />
                  </Button>
                </div>
              ))}
              {picking ? (
                <FolderPicker
                  start={paths.length ? parentOf(paths[paths.length - 1]) : undefined}
                  onChoose={(p) => {
                    if (!paths.includes(p)) setPaths([...paths, p]);
                    setPicking(false);
                  }}
                  onCancel={() => setPicking(false)}
                />
              ) : (
                <Button variant="secondary" size="sm" onClick={() => setPicking(true)}>
                  <FolderPlus /> Add Folder
                </Button>
              )}
            </div>
          </Field>
          <details open={editing || undefined} className="group rounded-2xl bg-surface-2/30 ring-1 ring-border">
            <summary className="flex cursor-pointer list-none items-center gap-3 rounded-2xl px-4 py-3 outline-none focus-visible:ring-2 focus-visible:ring-accent [&::-webkit-details-marker]:hidden">
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium">Advanced</span>
                <span className="block truncate text-xs text-subtle group-open:hidden">{advancedSummary}</span>
              </span>
              <ChevronDown className="size-4 shrink-0 text-subtle transition-transform group-open:rotate-180" />
            </summary>
            <div className="space-y-5 border-t border-border px-3 pb-4 pt-4 sm:px-4">
              {kind !== 'videos' && (
                <SettingGroup title="Metadata">
                  {kind === 'shows' && (
                    <SelectRow id={`${fieldId}-provider`} label="Metadata provider" help="Where show titles, descriptions, artwork and cast come from.">
                      <select id={`${fieldId}-provider`} aria-describedby={`${fieldId}-provider-help`} value={detailsProvider} onChange={(e) => setDetailsProvider(e.target.value as '' | 'tvdb')} className={selectClass}>
                        <option value="">TMDB</option>
                        <option value="tvdb">TheTVDB</option>
                      </select>
                    </SelectRow>
                  )}
                  {kind === 'shows' && (
                    <SelectRow
                      id={`${fieldId}-ordering`}
                      label="File episode order"
                      help={`${orderings.find((o) => o.value === ordering)?.hint}. Episode details come from the same provider. A show can override this in its Edit dialog; display order is separate.`}
                    >
                      <select id={`${fieldId}-ordering`} aria-describedby={`${fieldId}-ordering-help`} value={ordering} onChange={(e) => setOrdering(e.target.value as SourceOrdering)} className={selectClass}>
                        {orderings.map((o) => (
                          <option key={o.value} value={o.value}>
                            {o.label}
                          </option>
                        ))}
                      </select>
                    </SelectRow>
                  )}
                  <SelectRow
                    id={`${fieldId}-language`}
                    label="Metadata language"
                    help={
                      <>
                        {language && !language.startsWith('en')
                          ? 'Titles and descriptions in this language. Anything not translated yet is shown in English.'
                          : 'The language titles and descriptions are shown in.'}
                        {library && language !== (library.metadata_language ?? '') && ' Everything in the library is fetched again after you save.'}
                      </>
                    }
                  >
                    <select id={`${fieldId}-language`} aria-describedby={`${fieldId}-language-help`} value={language} onChange={(e) => setLanguage(e.target.value)} className={selectClass}>
                      {metadataLanguages.map(([code, name]) => (
                        <option key={code} value={code}>
                          {name}
                        </option>
                      ))}
                    </select>
                  </SelectRow>
                </SettingGroup>
              )}
              {kind !== 'videos' && (
                <SettingGroup title="Collections">
                  {kind === 'movies' && (
                    <SelectRow id={`${fieldId}-collection-min`} label="Automatic collections" help="Films TMDB groups together, like a trilogy, become a collection once you have this many.">
                      <select id={`${fieldId}-collection-min`} aria-describedby={`${fieldId}-collection-min-help`} value={collectionMin} onChange={(e) => setCollectionMin(Number(e.target.value))} className={selectClass}>
                        <option value={0}>Off</option>
                        {[1, 2, 3, 4, 5].map((n) => (
                          <option key={n} value={n}>
                            {n === 1 ? 'Any film from a set' : `${n} or more films`}
                          </option>
                        ))}
                      </select>
                    </SelectRow>
                  )}
                  <SelectRow id={`${fieldId}-collection-grid`} label="Collections in the library" help="How collections appear when you browse the whole library. Each collection can choose for itself.">
                    <select id={`${fieldId}-collection-grid`} aria-describedby={`${fieldId}-collection-grid-help`} value={collectionGrid || 'items'} onChange={(e) => setCollectionGrid(e.target.value === 'items' ? '' : (e.target.value as Library['collection_grid']))} className={selectClass}>
                      {(['items', 'collection', 'both'] as const).map((m) => (
                        <option key={m} value={m}>
                          {gridLabels[m]}
                        </option>
                      ))}
                    </select>
                  </SelectRow>
                </SettingGroup>
              )}
              <SettingGroup title="Player">
                <SwitchRow label="Scrubbing previews" help="A picture while dragging along the timeline. Made in the background, about 5 MB per film; turning this off deletes them." checked={trickplay} onChange={setTrickplay} />
                {kind === 'shows' && (
                  <SwitchRow label="Skip intros" help="Offer Skip Intro and Skip Recap, found from chapters, online databases or by comparing a season's episodes." checked={skipIntros} onChange={setSkipIntros} />
                )}
                {kind !== 'videos' && (
                  <SwitchRow
                    label="Skip credits"
                    help={kind === 'shows' ? 'Offer Skip Credits, and start the Up Next countdown when the credits begin.' : 'Offer Skip Credits, found from chapters, TheIntroDB or by spotting the closing credits at night.'}
                    checked={skipCredits}
                    onChange={setSkipCredits}
                  />
                )}
              </SettingGroup>
              {!editing && (
                <SettingGroup title="Before adding">
                  <div className="flex flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
                    <div className="min-w-0 flex-1 basis-56">
                      <p className="text-sm font-medium">Check the folders first</p>
                      <p className="mt-0.5 text-xs leading-relaxed text-subtle">Optional: see what Arc would find before adding the library.</p>
                    </div>
                    <Button variant="secondary" size="sm" disabled={!paths.length || !name.trim() || start.isPending || save.isPending} onClick={() => start.mutate()}>
                      {start.isPending && <Loader2 className="animate-spin" />} Preview folders
                    </Button>
                  </div>
                </SettingGroup>
              )}
            </div>
          </details>
        </div>
      )}
    </DialogContent>
  );
}

function parentOf(path: string) {
  const i = Math.max(path.lastIndexOf('\\'), path.lastIndexOf('/'));
  return i > 0 ? path.slice(0, i) : path;
}


function Field({ label, controlId, children }: { label: string; controlId?: string; children: ReactNode }) {
  return (
    <div>
      {controlId ? (
        <label htmlFor={controlId} className="mb-2 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{label}</label>
      ) : (
        <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">{label}</p>
      )}
      {children}
    </div>
  );
}

const selectClass = cn(inputClass, 'h-9 appearance-none pr-9');

/** A titled block of advanced settings rows. */
function SettingGroup({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title}>
      <h3 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">{title}</h3>
      <div className="divide-y divide-border overflow-hidden rounded-xl bg-surface-2/60 ring-1 ring-border">{children}</div>
    </section>
  );
}

/** A setting with its label and help on the left and a drop-down on the right (stacked on phones). */
function SelectRow({ id, label, help, children }: { id: string; label: string; help: ReactNode; children: ReactNode }) {
  return (
    <div className="grid gap-x-6 gap-y-2 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,17rem)] sm:items-center">
      <div className="min-w-0">
        <label htmlFor={id} className="block text-sm font-medium">{label}</label>
        <p id={`${id}-help`} className="mt-0.5 text-xs leading-relaxed text-subtle">{help}</p>
      </div>
      <div className="relative">
        {children}
        <ChevronDown className="pointer-events-none absolute right-3 top-2.5 size-4 text-subtle" />
      </div>
    </div>
  );
}

/** An on/off setting; the whole row toggles it. */
function SwitchRow({ label, help, checked, onChange }: { label: string; help: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-6 px-4 py-3">
      <span className="min-w-0">
        <span className="block text-sm font-medium">{label}</span>
        <span className="mt-0.5 block text-xs leading-relaxed text-subtle">{help}</span>
      </span>
      <Switch checked={checked} onCheckedChange={onChange} aria-label={label} />
    </label>
  );
}

function KindCard({ icon, label, selected, onSelect }: { icon: ReactNode; label: string; selected: boolean; onSelect: () => void }) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={cn(
        'flex items-center gap-3 rounded-2xl p-4 text-left text-sm font-medium ring-1 transition-all [&_svg]:size-5',
        selected ? 'bg-accent/10 text-foreground ring-2 ring-accent' : 'bg-surface-2/50 text-muted ring-border hover:text-foreground',
      )}
    >
      {icon} {label}
      {selected && <Check className="ml-auto text-accent" />}
    </button>
  );
}

function PreviewView({ id, kind }: { id: string; kind: LibraryKind }) {
  const preview = useQuery({
    queryKey: ['library-preview', id],
    queryFn: () => api.preview(id),
    refetchInterval: (q) => (q.state.data?.stage === 'done' ? false : 700),
    // Keep following progress while the window is in the background.
    refetchIntervalInBackground: true,
  });
  const [tab, setTab] = useState<'all' | 'unidentified' | 'unrecognised'>('all');
  const raw = preview.data;
  if (preview.error) return <p className="text-sm text-warning">{preview.error.message}</p>;
  if (!raw) return <div className="h-40 animate-pulse rounded-2xl bg-surface-2" />;
  // Defensive: an older server sent null for an empty list.
  const p = { ...raw, items: raw.items ?? [], unrecognised: raw.unrecognised ?? [], unavailable: raw.unavailable ?? [] };

  const what = kindUnit(kind)[0];
  const unidentified = p.items.filter((i) => i.status === 'unidentified');
  const identified = p.items.filter((i) => i.status === 'identified').length;
  const shown = tab === 'unidentified' ? unidentified : tab === 'all' ? p.items : [];

  return (
    <div className="space-y-5 pb-2">
      {p.stage !== 'done' && <Progress preview={p} what={what} />}
      {p.error && p.error !== 'stopped' && <p className="text-sm text-warning">{p.error}</p>}
      {p.unavailable.length > 0 && (
        <p className="flex items-start gap-2 rounded-xl bg-warning/10 px-3 py-2 text-sm text-warning">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" /> Couldn't open {p.unavailable.join(', ')}. Is the drive connected?
        </p>
      )}

      {kind === 'videos' ? (
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-2xl bg-border ring-1 ring-border">
          <Stat value={p.items.length} label={p.items.length === 1 ? what : `${what}s`} />
          <Stat value={p.folders ?? 0} label={p.folders === 1 ? 'folder' : 'folders'} />
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-2xl bg-border ring-1 ring-border sm:grid-cols-4">
          <Stat value={p.items.length} label={p.items.length === 1 ? what : `${what}s`} />
          <Stat value={identified} label="identified" tone="accent" />
          <Stat value={unidentified.length} label="not identified" tone={unidentified.length ? 'warning' : undefined} />
          <Stat value={p.unrecognised.length} label="files not recognised" tone={p.unrecognised.length ? 'warning' : undefined} />
        </div>
      )}
      <p className="text-xs text-subtle">
        {fmt.plural(p.files, 'video file')} found{p.extras > 0 ? `, plus ${fmt.plural(p.extras, 'extra')} (trailers, featurettes, samples) that are skipped` : ''}.
        {unidentified.length > 0 && ' Anything not identified can be fixed after adding, with Identify.'}
      </p>

      {p.stage === 'done' && p.files === 0 && p.unavailable.length === 0 && (
        <p className="flex items-start gap-2 rounded-xl bg-warning/10 px-3 py-2 text-sm text-warning">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" />
          <span>
            No video files in these folders. Check it's the folder with your videos in it. If Arc runs in Docker, the folder must be mounted
            into the container (e.g. /media/movies) and chosen by that name here.
          </span>
        </p>
      )}
      {p.items.length + p.unrecognised.length > 0 && (
        <div className="flex gap-1">
          <Tab active={tab === 'all'} onClick={() => setTab('all')}>
            All
          </Tab>
          {unidentified.length > 0 && (
            <Tab active={tab === 'unidentified'} onClick={() => setTab('unidentified')}>
              Not identified · {unidentified.length}
            </Tab>
          )}
          {p.unrecognised.length > 0 && (
            <Tab active={tab === 'unrecognised'} onClick={() => setTab('unrecognised')}>
              Not recognised · {p.unrecognised.length}
            </Tab>
          )}
        </div>
      )}

      <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface-2/40 ring-1 ring-border empty:hidden">
        {shown.map((item, i) => (
          <PreviewRow key={i} item={item} />
        ))}
        {tab === 'unrecognised' &&
          p.unrecognised.map((path) => (
            <div key={path} className="px-4 py-2.5">
              <p className="truncate text-sm">{path.split(/[\\/]/).pop()}</p>
              <p className="truncate font-mono text-[11px] text-subtle" title={path}>
                {path}
              </p>
            </div>
          ))}
      </div>
      {tab === 'unrecognised' && (
        <p className="text-xs text-subtle">
          {kind === 'shows'
            ? 'These files have no season and episode in their names (like “S01E02”), so Arc can’t place them.'
            : 'Arc couldn’t tell what these files are.'}
        </p>
      )}
    </div>
  );
}

function Progress({ preview: p, what }: { preview: LibraryPreview; what: string }) {
  const scanning = p.stage === 'scanning';
  const pct = scanning || !p.items.length ? 0 : Math.round((p.checked / p.items.length) * 100);
  return (
    <div>
      <p className="mb-2 flex items-center gap-2 text-sm text-muted">
        <Loader2 className="size-4 animate-spin text-accent" />
        {scanning ? `Looking through your folders… ${fmt.plural(p.files, 'video file')} so far` : `Identifying ${fmt.plural(p.items.length, what)} online… ${p.checked} done`}
      </p>
      <div className="h-1.5 overflow-hidden rounded-full bg-surface-3">
        <div className={cn('h-full rounded-full bg-accent transition-all duration-500', scanning && 'w-1/3 animate-pulse')} style={scanning ? undefined : { width: `${pct}%` }} />
      </div>
    </div>
  );
}

function Stat({ value, label, tone }: { value: number; label: string; tone?: 'accent' | 'warning' }) {
  return (
    <div className="bg-surface p-3.5">
      <p className={cn('text-2xl font-semibold tabular-nums', tone === 'accent' && 'text-accent', tone === 'warning' && 'text-warning')}>{value}</p>
      <p className="text-xs text-muted">{label}</p>
    </div>
  );
}

function Tab({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn('rounded-full px-3 py-1 text-[13px] font-medium transition-colors', active ? 'bg-foreground text-background' : 'text-muted hover:text-foreground')}
    >
      {children}
    </button>
  );
}

function PreviewRow({ item }: { item: PreviewItem }) {
  const [open, setOpen] = useState(false);
  const m = item.match;
  const provider = m?.provider === 'tvdb' ? 'TheTVDB' : 'TMDB';
  const count = item.kind === 'series' ? fmt.plural(item.episodes ?? 0, 'episode') : item.files > 1 ? fmt.plural(item.files, 'file') : '';
  return (
    <div>
      <div className="flex items-center gap-3.5 px-3 py-2.5">
        <div className="aspect-[2/3] w-9 shrink-0 overflow-hidden rounded-md bg-surface-3">
          {m?.poster && <img src={m.poster} alt="" loading="lazy" className="size-full object-cover" />}
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">
            {m ? m.title : item.title}
            {(m?.year || item.year) && <span className="font-normal text-muted"> ({m?.year || item.year})</span>}
          </p>
          <p className="truncate text-xs text-subtle">
            {[count, `from “${item.example.source}”`].filter(Boolean).join(' · ')}
          </p>
        </div>
        {item.status === 'pending' && <Loader2 className="size-4 shrink-0 animate-spin text-subtle" />}
        {item.status === 'identified' && (
          <span className="flex shrink-0 items-center gap-1 text-xs text-accent">
            <Check className="size-3.5" /> {provider}
          </span>
        )}
        {item.status === 'unidentified' && (
          <button type="button" onClick={() => setOpen(!open)} className="flex shrink-0 items-center gap-1 text-xs font-medium text-warning hover:underline">
            <HelpCircle className="size-3.5" /> Why?
          </button>
        )}
      </div>
      {open && (
        <ol className="space-y-2.5 border-t border-border bg-surface/60 px-4 py-3">
          <ParsedStep parsed={item.example} kind={item.kind} />
          {item.steps.map((s, i) => (
            <StepRow key={i} step={s} />
          ))}
        </ol>
      )}
    </div>
  );
}

// Languages TMDB has good coverage in; "" is TMDB's default, American English.
const metadataLanguages: [string, string][] = [
  ['', 'English (US)'],
  ['en-GB', 'English (UK)'],
  ['ar-SA', 'Arabic'],
  ['zh-CN', 'Chinese (Simplified)'],
  ['zh-TW', 'Chinese (Traditional)'],
  ['cs-CZ', 'Czech'],
  ['da-DK', 'Danish'],
  ['nl-NL', 'Dutch'],
  ['fi-FI', 'Finnish'],
  ['fr-FR', 'French'],
  ['fr-CA', 'French (Canada)'],
  ['de-DE', 'German'],
  ['el-GR', 'Greek'],
  ['he-IL', 'Hebrew'],
  ['hi-IN', 'Hindi'],
  ['hu-HU', 'Hungarian'],
  ['id-ID', 'Indonesian'],
  ['it-IT', 'Italian'],
  ['ja-JP', 'Japanese'],
  ['ko-KR', 'Korean'],
  ['nb-NO', 'Norwegian'],
  ['pl-PL', 'Polish'],
  ['pt-BR', 'Portuguese (Brazil)'],
  ['pt-PT', 'Portuguese (Portugal)'],
  ['ro-RO', 'Romanian'],
  ['ru-RU', 'Russian'],
  ['es-ES', 'Spanish (Spain)'],
  ['es-MX', 'Spanish (Latin America)'],
  ['sv-SE', 'Swedish'],
  ['th-TH', 'Thai'],
  ['tr-TR', 'Turkish'],
  ['uk-UA', 'Ukrainian'],
  ['vi-VN', 'Vietnamese'],
];
