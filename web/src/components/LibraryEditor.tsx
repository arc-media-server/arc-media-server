import { useEffect, useState, type ReactNode } from 'react';
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
  { value: 'tmdb_aired', label: 'TMDB aired order', hint: 'Files numbered the way TMDB lists them; the default, as in Plex' },
  { value: 'tvdb_aired', label: 'TheTVDB aired order', hint: "Files numbered the way TheTVDB lists them (Sonarr's default)" },
  { value: 'tvdb_dvd', label: 'TheTVDB DVD order', hint: 'Files numbered like the DVD releases' },
  { value: 'tvdb_absolute', label: 'TheTVDB absolute order', hint: 'Episodes numbered 1, 2, 3… across seasons (common for anime)' },
];

/** Adds a library (choose folders, preview what Arc finds, add), or edits one when `library` is set. */
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
  const [kind, setKind] = useState<LibraryKind>(library?.kind ?? 'movies');
  const [name, setName] = useState(library?.name ?? '');
  const [nameTouched, setNameTouched] = useState(editing);
  const [paths, setPaths] = useState<string[]>(library?.paths ?? []);
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

  const start = useMutation({
    mutationFn: () => api.startPreview({ kind, source_ordering: ordering, paths }, library?.id),
    onSuccess: (res) => {
      setPreviewId(res.id);
      setStep('preview');
    },
  });
  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.updateLibrary(library.id, { name, source_ordering: ordering, paths, trickplay, collection_min: collectionMin, metadata_language: language, skip_intros: skipIntros, skip_credits: skipCredits, collection_grid: collectionGrid })
        : api.createLibrary({ name, kind, source_ordering: ordering, paths, trickplay, collection_min: collectionMin, metadata_language: language, skip_intros: skipIntros, skip_credits: skipCredits, collection_grid: collectionGrid }),
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
        {editing ? (
          <Button variant="primary" disabled={!paths.length || !name.trim() || save.isPending} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="animate-spin" />} Save
          </Button>
        ) : (
          <Button variant="primary" disabled={!paths.length || !name.trim() || start.isPending} onClick={() => start.mutate()}>
            {start.isPending && <Loader2 className="animate-spin" />} Preview
          </Button>
        )}
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
            ? 'Change the name or folders. Arc scans the library again when you save.'
            : 'Choose what kind of library this is and where its files are.'
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
              <div className="grid grid-cols-3 gap-3">
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
          <Field label="Name">
            <input
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
          {kind === 'shows' && (
            <Field label="Episode numbering in your file names">
              <div className="relative">
                <select value={ordering} onChange={(e) => setOrdering(e.target.value as SourceOrdering)} className={cn(inputClass, 'appearance-none pr-9')}>
                  {orderings.map((o) => (
                    <option key={o.value} value={o.value}>
                      {o.label}
                    </option>
                  ))}
                </select>
                <ChevronDown className="pointer-events-none absolute right-3 top-3 size-4 text-subtle" />
              </div>
              <p className="mt-1.5 text-xs text-subtle">
                {orderings.find((o) => o.value === ordering)?.hint}. This is only how files are named; you can show each series in any
                order later.
              </p>
            </Field>
          )}
          {kind === 'movies' && (
            <Field label="Automatic collections">
              <select value={collectionMin} onChange={(e) => setCollectionMin(Number(e.target.value))} className={inputClass}>
                <option value={0}>Off</option>
                {[1, 2, 3, 4, 5].map((n) => (
                  <option key={n} value={n}>
                    When I have {n === 1 ? 'any film' : `${n} or more films`} from one
                  </option>
                ))}
              </select>
              <p className="mt-1.5 text-xs text-subtle">Films TMDB groups together, like a trilogy, are shown as a collection.</p>
            </Field>
          )}
          {kind !== 'videos' && (
            <Field label="Collections in the library">
              <select value={collectionGrid || 'items'} onChange={(e) => setCollectionGrid(e.target.value === 'items' ? '' : (e.target.value as Library['collection_grid']))} className={inputClass}>
                {(['items', 'collection', 'both'] as const).map((m) => (
                  <option key={m} value={m}>
                    {gridLabels[m]}
                  </option>
                ))}
              </select>
              <p className="mt-1.5 text-xs text-subtle">How a collection appears when you browse the whole library. Each collection can choose for itself in its Edit dialog.</p>
            </Field>
          )}
          {kind !== 'videos' && (
          <Field label="Metadata language">
            <div className="relative">
              <select value={language} onChange={(e) => setLanguage(e.target.value)} className={cn(inputClass, 'appearance-none pr-9')}>
                {metadataLanguages.map(([code, name]) => (
                  <option key={code} value={code}>
                    {name}
                  </option>
                ))}
              </select>
              <ChevronDown className="pointer-events-none absolute right-3 top-3 size-4 text-subtle" />
            </div>
            <p className="mt-1.5 text-xs text-subtle">
              {language && !language.startsWith('en')
                ? 'Titles and descriptions in this language. Anything not translated yet is shown in English.'
                : 'The language titles and descriptions are shown in.'}
              {library && language !== (library.metadata_language ?? '') && ' Everything in the library is fetched again after you save.'}
            </p>
          </Field>
          )}
          <label className="flex cursor-pointer items-start justify-between gap-4 rounded-2xl bg-surface-2/60 p-4 ring-1 ring-border">
            <span>
              <span className="block text-sm font-medium">Scrubbing previews</span>
              <span className="mt-0.5 block text-xs leading-relaxed text-subtle">
                Show a picture while dragging along the timeline. Arc makes them in the background, which takes a little time and about
                5 MB per film. Turning this off deletes them.
              </span>
            </span>
            <Switch checked={trickplay} onCheckedChange={setTrickplay} aria-label="Scrubbing previews" />
          </label>
          {kind === 'shows' && (
            <label className="flex cursor-pointer items-start justify-between gap-4 rounded-2xl bg-surface-2/60 p-4 ring-1 ring-border">
              <span>
                <span className="block text-sm font-medium">Skip intros</span>
                <span className="mt-0.5 block text-xs leading-relaxed text-subtle">
                  Offer Skip Intro and Skip Recap. Arc finds them from chapters, online databases, or by comparing a season&apos;s episodes.
                </span>
              </span>
              <Switch checked={skipIntros} onCheckedChange={setSkipIntros} aria-label="Skip intros" />
            </label>
          )}
          {kind !== 'videos' && (
            <label className="flex cursor-pointer items-start justify-between gap-4 rounded-2xl bg-surface-2/60 p-4 ring-1 ring-border">
              <span>
                <span className="block text-sm font-medium">Skip credits</span>
                <span className="mt-0.5 block text-xs leading-relaxed text-subtle">
                  {kind === 'shows'
                    ? 'Offer Skip Credits, and start the Up Next countdown when the credits begin.'
                    : 'Offer Skip Credits. Arc finds them from chapters, TheIntroDB, or by spotting the closing credits at night.'}
                </span>
              </span>
              <Switch checked={skipCredits} onCheckedChange={setSkipCredits} aria-label="Skip credits" />
            </label>
          )}
        </div>
      )}
    </DialogContent>
  );
}

function parentOf(path: string) {
  const i = Math.max(path.lastIndexOf('\\'), path.lastIndexOf('/'));
  return i > 0 ? path.slice(0, i) : path;
}


function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-subtle">{label}</p>
      {children}
    </div>
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
