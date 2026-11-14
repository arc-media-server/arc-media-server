import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { useEffect, useId, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Check, ChevronDown, Film, Folder, FolderPlus, HelpCircle, Info, Loader2, SlidersHorizontal, Tv, X, Clapperboard } from 'lucide-react';
import { regionOptions } from '@/lib/regions';
import { api, type Library, type LibraryKind, type LibraryPreview, type PreviewItem, type SourceOrdering } from '@/lib/api';
import { cn } from '@/lib/utils';
import { kindUnit } from '@/lib/kinds';
import { FolderPicker } from '@/components/FolderPicker';
import { ParsedStep, StepRow } from '@/components/FixMatch';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';
import { Switch } from '@/components/ui/switch';

export const orderings: { value: SourceOrdering; label: string }[] = [
  { value: 'tmdb_aired', get label() { return i18n.t('libraryEditor:sourceOrder.tmdbAired'); } },
  { value: 'tvdb_aired', get label() { return i18n.t('libraryEditor:sourceOrder.tvdbAired'); } },
  { value: 'tvdb_dvd', get label() { return i18n.t('libraryEditor:sourceOrder.tvdbDvd'); } },
  { value: 'tvdb_absolute', get label() { return i18n.t('libraryEditor:sourceOrder.tvdbAbsolute'); } },
];

/** Adds a library for background scanning; a folder preview is optional. */
export function LibraryEditorDialog({ library, open, onOpenChange, onAdded }: { library?: Library; open: boolean; onOpenChange: (open: boolean) => void; onAdded?: (library: Library) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <Editor library={library} onAdded={onAdded} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function Editor({ library, onDone, onAdded }: { library?: Library; onDone: () => void; onAdded?: (library: Library) => void }) {
  const { t } = useTranslation('libraryEditor');
  const qc = useQueryClient();
  const gridLabels = { items: t('libraryEditor:grid.items'), collection: t('libraryEditor:grid.collection'), both: t('libraryEditor:grid.both') };
  const sourceHints = {
    tmdb_aired: t('libraryEditor:sourceHints.tmdbAired'),
    tvdb_aired: t('libraryEditor:sourceHints.tvdbAired'),
    tvdb_dvd: t('libraryEditor:sourceHints.tvdbDvd'),
    tvdb_absolute: t('libraryEditor:sourceHints.tvdbAbsolute'),
  };
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
  // New libraries start with the server's country (set from the browser at setup), as Plex's
  // per-library Certification Country.
  const serverRegion = useQuery({ queryKey: ['settings'], queryFn: api.settings }).data?.region;
  const [ratingRegion, setRatingRegion] = useState(library?.rating_region ?? '');
  const region = ratingRegion || serverRegion || 'US';
  const [picking, setPicking] = useState(!editing);
  const [step, setStep] = useState<'details' | 'preview'>('details');
  const [previewId, setPreviewId] = useState<string | null>(null);

  // Name the library after its kind until the user names it themselves.
  useEffect(() => {
    if (!nameTouched) setName(kind === 'shows' ? t('libraryEditor:defaults.shows') : kind === 'videos' ? t('libraryEditor:defaults.videos') : t('libraryEditor:defaults.movies'));
  }, [kind, nameTouched, t]);

  const [section, setSection] = useState<Section>('general');
  const sections: NavSection[] = [
    { id: 'general', label: t('libraryEditor:navigation.general'), icon: <Info /> },
    { id: 'folders', label: t('libraryEditor:navigation.folders'), icon: <Folder />, note: paths.length ? String(paths.length) : undefined, attention: !paths.length },
    { id: 'advanced', label: t('libraryEditor:navigation.advanced'), icon: <SlidersHorizontal /> },
  ];
  // While adding, lead through the folders before offering Add Library.
  const needsFolders = !editing && !paths.length && section !== 'folders';

  const start = useMutation({
    mutationFn: () => api.startPreview({ kind, source_ordering: ordering, paths, metadata_language: language, rating_region: region }, library?.id),
    onSuccess: (res) => {
      setPreviewId(res.id);
      setStep('preview');
    },
  });
  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.updateLibrary(library.id, { name, details_provider: kind==='shows'?detailsProvider:"", source_ordering: ordering, paths, trickplay, collection_min: collectionMin, metadata_language: language, rating_region: region, skip_intros: skipIntros, skip_credits: skipCredits, collection_grid: collectionGrid })
        : api.createLibrary({ name, kind, details_provider: kind==='shows'?detailsProvider:"", source_ordering: ordering, paths, trickplay, collection_min: collectionMin, metadata_language: language, rating_region: region, skip_intros: skipIntros, skip_credits: skipCredits, collection_grid: collectionGrid }),
    onSuccess: (saved) => {
      qc.invalidateQueries();
      if (!editing) onAdded?.(saved);
      onDone();
    },
  });

  const footer =
    step === 'details' ? (
      <>
        {(start.error || save.error) && <p className="mr-auto text-sm text-warning">{(start.error ?? save.error)!.message}</p>}
        <DialogClose asChild>
          <Button variant="ghost">{t("libraryEditor:libraryEditor.cancel")}</Button>
        </DialogClose>
        {needsFolders ? (
          <Button variant="primary" disabled={!name.trim()} onClick={() => setSection('folders')}>{t("libraryEditor:libraryEditor.next")}</Button>
        ) : (
          <Button variant="primary" disabled={!paths.length || !name.trim() || save.isPending || start.isPending} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="animate-spin" />} {editing ? t("libraryEditor:libraryEditor.save") : t("libraryEditor:libraryEditor.addLibrary")}
          </Button>
        )}
      </>
    ) : (
      <>
        {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
        <Button variant="ghost" onClick={() => setStep('details')} disabled={save.isPending}>{t("libraryEditor:libraryEditor.back")}</Button>
        <Button variant="primary" onClick={() => save.mutate()} disabled={save.isPending}>
          {save.isPending && <Loader2 className="animate-spin" />} {t("libraryEditor:libraryEditor.addLibrary")}</Button>
      </>
    );

  return (
    <DialogContent
      title={editing ? t('libraryEditor:libraryEditor.editTitle', { name: library.name }) : step === 'preview' ? t('libraryEditor:libraryEditor.previewTitle', { name }) : t("libraryEditor:libraryEditor.addALibrary")}
      description={
        step === 'preview'
          ? t("libraryEditor:libraryEditor.thisIsWhatArc")
          : editing
            ? t("libraryEditor:libraryEditor.folderChangesScanThe")
            : t("libraryEditor:libraryEditor.chooseATypeAnd")
      }
      footer={footer}
      className="w-[min(820px,calc(100vw-32px))]"
    >
      {step === 'preview' && previewId ? (
        <PreviewView id={previewId} kind={kind} />
      ) : (
        <div className="grid gap-5 sm:grid-cols-[168px_minmax(0,1fr)] sm:gap-7">
          <SectionNav sections={sections} current={section} onSelect={setSection} idBase={fieldId} />
          <div role="tabpanel" id={`${fieldId}-${section}-panel`} aria-labelledby={`${fieldId}-${section}-tab`} className="min-w-0 pb-2 sm:min-h-[420px]">
            {section === 'general' && (
              <div className="space-y-6">
                <PanelHeading title={t("libraryEditor:libraryEditor.general")} description={editing ? t("libraryEditor:libraryEditor.whatThisLibraryIs") : t("libraryEditor:libraryEditor.whatKindOfVideos")} />
                {editing ? (
                  <Field label={t("libraryEditor:libraryEditor.type")}>
                    <p className="flex items-center gap-2 text-sm text-muted [&_svg]:size-4">{kind === 'shows' ? <Tv /> : kind === 'videos' ? <Clapperboard /> : <Film />} {kind === 'shows' ? t("libraryEditor:libraryEditor.tvShows") : kind === 'videos' ? t("libraryEditor:libraryEditor.otherVideos") : t("libraryEditor:libraryEditor.movies")}</p>
                  </Field>
                ) : (
                  <Field label={t("libraryEditor:libraryEditor.type")}>
                    <div className="grid gap-3 sm:grid-cols-3">
                      <KindCard icon={<Film />} label={t("libraryEditor:libraryEditor.movies")} selected={kind === 'movies'} onSelect={() => setKind('movies')} />
                      <KindCard icon={<Tv />} label={t("libraryEditor:libraryEditor.tvShows")} selected={kind === 'shows'} onSelect={() => setKind('shows')} />
                      <KindCard icon={<Clapperboard />} label={t("libraryEditor:libraryEditor.otherVideos")} selected={kind === 'videos'} onSelect={() => setKind('videos')} />
                    </div>
                    {kind === 'videos' && (
                      <p className="mt-1.5 text-xs text-subtle">{t("libraryEditor:libraryEditor.homeVideosAndAnything")}</p>
                    )}
                  </Field>
                )}
                <Field label={t("libraryEditor:libraryEditor.name")} controlId={`${fieldId}-name`}>
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
              </div>
            )}
            {section === 'folders' && (
              <div className="space-y-4">
                <PanelHeading title={t("libraryEditor:libraryEditor.folders")} description={t("libraryEditor:libraryEditor.arcLooksInThese")} />
                <div className="space-y-2">
                  {paths.map((p) => (
                    <div key={p} className="flex items-center gap-2 rounded-xl bg-surface-2/80 py-1.5 pl-3 pr-1.5 ring-1 ring-border">
                      <span className="min-w-0 flex-1 truncate font-mono text-xs" title={p}>
                        {p}
                      </span>
                      <Button variant="ghost" size="icon" className="size-7 rounded-lg" aria-label={t('libraryEditor:libraryEditor.removePath', { path: p })} onClick={() => setPaths(paths.filter((x) => x !== p))}>
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
                      <FolderPlus /> {t("libraryEditor:libraryEditor.addFolder")}</Button>
                  )}
                </div>
              </div>
            )}
            {section === 'advanced' && (
              <div className="space-y-5">
                <PanelHeading title={t("libraryEditor:libraryEditor.advanced")} description={editing ? t("libraryEditor:libraryEditor.metadataCollectionsAndPlayer") : t("libraryEditor:libraryEditor.optionalTheDefaultsSuit")} />
                {kind !== 'videos' && (
                  <SettingGroup title={t("libraryEditor:libraryEditor.metadata")}>
                    {kind === 'shows' && (
                      <SelectRow id={`${fieldId}-provider`} label={t("libraryEditor:libraryEditor.metadataProvider")} help={t('libraryEditor:help.metadataProvider')}>
                        <select id={`${fieldId}-provider`} aria-describedby={`${fieldId}-provider-help`} value={detailsProvider} onChange={(e) => setDetailsProvider(e.target.value as '' | 'tvdb')} className={selectClass}>
                          <option value="">{t("libraryEditor:libraryEditor.tmdb")}</option>
                          <option value="tvdb">{t("libraryEditor:libraryEditor.thetvdb")}</option>
                        </select>
                      </SelectRow>
                    )}
                    {kind === 'shows' && (
                      <SelectRow
                        id={`${fieldId}-ordering`}
                        label={t("libraryEditor:libraryEditor.fileEpisodeOrder")}
                        help={sourceHints[ordering]}
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
                      label={t("libraryEditor:libraryEditor.metadataLanguage")}
                      help={
                        <>
                          {language && !language.startsWith('en')
                            ? t("libraryEditor:libraryEditor.titlesAndDescriptionsIn")
                            : t("libraryEditor:libraryEditor.theLanguageTitlesAnd")}
                          {library && language !== (library.metadata_language ?? '') && t("libraryEditor:libraryEditor.everythingInTheLibrary")}
                        </>
                      }
                    >
                      <select id={`${fieldId}-language`} aria-describedby={`${fieldId}-language-help`} value={language} onChange={(e) => setLanguage(e.target.value)} className={selectClass}>
                        {metadataLanguages().map(([code, name]) => (
                          <option key={code} value={code}>
                            {name}
                          </option>
                        ))}
                      </select>
                    </SelectRow>
                    <SelectRow
                      id={`${fieldId}-ratings`}
                      label={t("libraryEditor:libraryEditor.ageRatings")}
                      help={
                        <>{t("libraryEditor:libraryEditor.whichCountrySAge")}{library && region !== library.rating_region && t("libraryEditor:libraryEditor.detailsAreFetchedAgain")}
                        </>
                      }
                    >
                      <select id={`${fieldId}-ratings`} aria-describedby={`${fieldId}-ratings-help`} value={region} onChange={(e) => setRatingRegion(e.target.value)} className={selectClass}>
                        {regionOptions(region).map(([code, name]) => (
                          <option key={code} value={code}>
                            {name}
                          </option>
                        ))}
                      </select>
                    </SelectRow>
                  </SettingGroup>
                )}
                {kind !== 'videos' && (
                  <SettingGroup title={t("libraryEditor:libraryEditor.collections")}>
                    {kind === 'movies' && (
                      <SelectRow id={`${fieldId}-collection-min`} label={t("libraryEditor:libraryEditor.automaticCollections")} help={t('libraryEditor:help.automaticCollections')}>
                        <select id={`${fieldId}-collection-min`} aria-describedby={`${fieldId}-collection-min-help`} value={collectionMin} onChange={(e) => setCollectionMin(Number(e.target.value))} className={selectClass}>
                          <option value={0}>{t("libraryEditor:libraryEditor.off")}</option>
                          {[1, 2, 3, 4, 5].map((n) => (
                            <option key={n} value={n}>
                              {n === 1 ? t("libraryEditor:libraryEditor.anyFilmFromA") : t('libraryEditor:libraryEditor.collectionThreshold', { count: n })}
                            </option>
                          ))}
                        </select>
                      </SelectRow>
                    )}
                    <SelectRow id={`${fieldId}-collection-grid`} label={t("libraryEditor:libraryEditor.collectionsInTheLibrary")} help={t('libraryEditor:help.collectionGrid')}>
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
                <SettingGroup title={t("libraryEditor:libraryEditor.player")}>
                  <SwitchRow label={t("libraryEditor:libraryEditor.scrubbingPreviews")} help={t('libraryEditor:help.scrubbingPreviews')} checked={trickplay} onChange={setTrickplay} />
                  {kind === 'shows' && (
                    <SwitchRow label={t("libraryEditor:libraryEditor.skipIntros")} help={t('libraryEditor:help.skipMarkers')} checked={skipIntros} onChange={setSkipIntros} />
                  )}
                  {kind !== 'videos' && (
                    <SwitchRow
                      label={t("libraryEditor:libraryEditor.skipCredits")}
                      help={kind === 'shows' ? t('libraryEditor:help.showCredits') : t('libraryEditor:help.movieCredits')}
                      checked={skipCredits}
                      onChange={setSkipCredits}
                    />
                  )}
                </SettingGroup>
                {!editing && (
                  <SettingGroup title={t("libraryEditor:libraryEditor.beforeAdding")}>
                    <div className="flex flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3">
                      <div className="min-w-0 flex-1 basis-56">
                        <p className="text-sm font-medium">{t("libraryEditor:libraryEditor.checkTheFoldersFirst")}</p>
                        <p className="mt-0.5 text-xs leading-relaxed text-subtle">{t("libraryEditor:libraryEditor.optionalSeeWhatArc")}</p>
                      </div>
                      <Button variant="secondary" size="sm" disabled={!paths.length || !name.trim() || start.isPending || save.isPending} onClick={() => start.mutate()}>
                        {start.isPending && <Loader2 className="animate-spin" />} {t("libraryEditor:libraryEditor.previewFolders")}</Button>
                    </div>
                  </SettingGroup>
                )}
              </div>
            )}
          </div>
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

type Section = 'general' | 'folders' | 'advanced';
type NavSection = { id: Section; label: string; icon: ReactNode; note?: string; attention?: boolean };

/** The editor's sections: a sidebar on wider screens, tabs across the top on phones. */
function SectionNav({ sections, current, onSelect, idBase }: { sections: NavSection[]; current: Section; onSelect: (s: Section) => void; idBase: string }) {
  const { t } = useTranslation('libraryEditor');
  const move = (from: number, by: number) => {
    const next = sections[(from + by + sections.length) % sections.length];
    onSelect(next.id);
    document.getElementById(`${idBase}-${next.id}-tab`)?.focus();
  };
  return (
    <div role="tablist" aria-label={t("libraryEditor:libraryEditor.librarySettings")} className="flex gap-1 rounded-xl bg-surface-2/50 p-1 ring-1 ring-border sm:flex-col sm:self-start sm:bg-transparent sm:p-0 sm:ring-0">
      {sections.map((s, i) => (
        <button
          key={s.id}
          type="button"
          role="tab"
          id={`${idBase}-${s.id}-tab`}
          aria-selected={current === s.id}
          aria-controls={`${idBase}-${s.id}-panel`}
          tabIndex={current === s.id ? 0 : -1}
          onClick={() => onSelect(s.id)}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown' || e.key === 'ArrowRight') { e.preventDefault(); move(i, 1); }
            if (e.key === 'ArrowUp' || e.key === 'ArrowLeft') { e.preventDefault(); move(i, -1); }
          }}
          className={cn(
            'flex flex-1 items-center justify-center gap-2 rounded-lg px-2 py-2 text-sm font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-accent sm:flex-none sm:justify-start sm:px-3 [&_svg]:size-4 max-sm:[&_svg]:hidden',
            current === s.id ? 'bg-surface-2 text-foreground ring-1 ring-border' : 'text-muted hover:text-foreground',
          )}
        >
          {s.icon}
          <span>{s.label}</span>
          {s.note && <span className="ml-auto hidden rounded-full bg-surface px-1.5 text-[11px] text-subtle sm:inline">{s.note}</span>}
          {s.attention && <span aria-label={t("libraryEditor:libraryEditor.needsAFolder")} className="ml-auto size-1.5 shrink-0 rounded-full bg-accent max-sm:ml-0" />}
        </button>
      ))}
    </div>
  );
}

function PanelHeading({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <h3 className="text-base font-semibold">{title}</h3>
      <p className="mt-0.5 text-sm text-muted">{description}</p>
    </div>
  );
}

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
  const { t } = useTranslation('libraryEditor');
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
          <AlertTriangle className="mt-0.5 size-4 shrink-0" /> {t('libraryEditor:preview.unavailable', { paths: p.unavailable.join(', ') })}</p>
      )}

      {kind === 'videos' ? (
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-2xl bg-border ring-1 ring-border">
          <Stat value={p.items.length} label={unitNoun(what, p.items.length)} />
          <Stat value={p.folders ?? 0} label={t('libraryEditor:preview.folderNoun', { count: p.folders ?? 0 })} />
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-2xl bg-border ring-1 ring-border sm:grid-cols-4">
          <Stat value={p.items.length} label={unitNoun(what, p.items.length)} />
          <Stat value={identified} label={t("libraryEditor:libraryEditor.identified")} tone="accent" />
          <Stat value={unidentified.length} label={t("libraryEditor:libraryEditor.notIdentified")} tone={unidentified.length ? 'warning' : undefined} />
          <Stat value={p.unrecognised.length} label={t("libraryEditor:libraryEditor.filesNotRecognised")} tone={p.unrecognised.length ? 'warning' : undefined} />
        </div>
      )}
      <p className="text-xs text-subtle">
        {p.extras > 0 ? t('libraryEditor:preview.foundExtras', { count: p.files, extras: t('libraryEditor:preview.extras', { count: p.extras }) }) : t('libraryEditor:preview.foundFiles', { count: p.files })}
        {unidentified.length > 0 && t("libraryEditor:libraryEditor.anythingNotIdentifiedCan")}
      </p>

      {p.stage === 'done' && p.files === 0 && p.unavailable.length === 0 && (
        <p className="flex items-start gap-2 rounded-xl bg-warning/10 px-3 py-2 text-sm text-warning">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" />
          <span>{t("libraryEditor:libraryEditor.noVideoFilesIn")}</span>
        </p>
      )}
      {p.items.length + p.unrecognised.length > 0 && (
        <div className="flex gap-1">
          <Tab active={tab === 'all'} onClick={() => setTab('all')}>{t("libraryEditor:libraryEditor.all")}</Tab>
          {unidentified.length > 0 && (
            <Tab active={tab === 'unidentified'} onClick={() => setTab('unidentified')}>{t('libraryEditor:preview.unidentifiedTab', { count: unidentified.length })}
            </Tab>
          )}
          {p.unrecognised.length > 0 && (
            <Tab active={tab === 'unrecognised'} onClick={() => setTab('unrecognised')}>{t('libraryEditor:preview.unrecognisedTab', { count: p.unrecognised.length })}
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
            ? t("libraryEditor:libraryEditor.theseFilesHaveNo")
            : t("libraryEditor:libraryEditor.arcCouldnTTell")}
        </p>
      )}
    </div>
  );
}

function Progress({ preview: p, what }: { preview: LibraryPreview; what: string }) {
  const { t } = useTranslation('libraryEditor');
  const scanning = p.stage === 'scanning';
  const pct = scanning || !p.items.length ? 0 : Math.round((p.checked / p.items.length) * 100);
  return (
    <div>
      <p className="mb-2 flex items-center gap-2 text-sm text-muted">
        <Loader2 className="size-4 animate-spin text-accent" />
        {scanning ? t('libraryEditor:preview.scanning', { count: p.files }) : what === 'show' ? t('libraryEditor:preview.identifyingShows', { count: p.items.length, checked: p.checked }) : what === 'video' ? t('libraryEditor:preview.identifyingVideos', { count: p.items.length, checked: p.checked }) : t('libraryEditor:preview.identifyingMovies', { count: p.items.length, checked: p.checked })}
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
  const { t } = useTranslation('libraryEditor');
  const [open, setOpen] = useState(false);
  const m = item.match;
  const provider = m?.provider === 'tvdb' ? 'TheTVDB' : 'TMDB';
  const count = item.kind === 'series' ? t('libraryEditor:preview.episodeCount', { count: item.episodes ?? 0 }) : item.files > 1 ? t('libraryEditor:preview.fileCount', { count: item.files }) : '';
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
            {[count, t('libraryEditor:preview.source', { name: item.example.source })].filter(Boolean).join(' · ')}
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
            <HelpCircle className="size-3.5" /> {t("libraryEditor:libraryEditor.why")}</button>
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
function metadataLanguages(): [string, string][] {
  return [
    ["", i18n.t('libraryEditor:languages.englishUs')],
    ["en-GB", i18n.t('libraryEditor:languages.englishUk')],
    ["ar-SA", i18n.t('libraryEditor:languages.arabic')],
    ["zh-CN", i18n.t('libraryEditor:languages.chineseSimplified')],
    ["zh-TW", i18n.t('libraryEditor:languages.chineseTraditional')],
    ["cs-CZ", i18n.t('libraryEditor:languages.czech')],
    ["da-DK", i18n.t('libraryEditor:languages.danish')],
    ["nl-NL", i18n.t('libraryEditor:languages.dutch')],
    ["fi-FI", i18n.t('libraryEditor:languages.finnish')],
    ["fr-FR", i18n.t('libraryEditor:languages.french')],
    ["fr-CA", i18n.t('libraryEditor:languages.frenchCanada')],
    ["de-DE", i18n.t('libraryEditor:languages.german')],
    ["el-GR", i18n.t('libraryEditor:languages.greek')],
    ["he-IL", i18n.t('libraryEditor:languages.hebrew')],
    ["hi-IN", i18n.t('libraryEditor:languages.hindi')],
    ["hu-HU", i18n.t('libraryEditor:languages.hungarian')],
    ["id-ID", i18n.t('libraryEditor:languages.indonesian')],
    ["it-IT", i18n.t('libraryEditor:languages.italian')],
    ["ja-JP", i18n.t('libraryEditor:languages.japanese')],
    ["ko-KR", i18n.t('libraryEditor:languages.korean')],
    ["nb-NO", i18n.t('libraryEditor:languages.norwegian')],
    ["pl-PL", i18n.t('libraryEditor:languages.polish')],
    ["pt-BR", i18n.t('libraryEditor:languages.portugueseBrazil')],
    ["pt-PT", i18n.t('libraryEditor:languages.portuguesePortugal')],
    ["ro-RO", i18n.t('libraryEditor:languages.romanian')],
    ["ru-RU", i18n.t('libraryEditor:languages.russian')],
    ["es-ES", i18n.t('libraryEditor:languages.spanishSpain')],
    ["es-MX", i18n.t('libraryEditor:languages.spanishLatinAmerica')],
    ["sv-SE", i18n.t('libraryEditor:languages.swedish')],
    ["th-TH", i18n.t('libraryEditor:languages.thai')],
    ["tr-TR", i18n.t('libraryEditor:languages.turkish')],
    ["uk-UA", i18n.t('libraryEditor:languages.ukrainian')],
    ["vi-VN", i18n.t('libraryEditor:languages.vietnamese')],
  ];
}

function unitNoun(unit: string, count: number) {
  if (unit === 'show') return i18n.t('libraryEditor:preview.showNoun', { count });
  if (unit === 'video') return i18n.t('libraryEditor:preview.videoNoun', { count });
  if (unit === 'movie') return i18n.t('libraryEditor:preview.movieNoun', { count });
  return '';
}
