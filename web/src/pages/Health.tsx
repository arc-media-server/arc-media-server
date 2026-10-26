import { useState, type ReactNode } from 'react';
import { Link } from 'react-router';
import { useQueries } from '@tanstack/react-query';
import {
  CheckCircle2,
  ChevronRight,
  Copy,
  FileQuestion,
  FileWarning,
  FileX2,
  HelpCircle,
  ListOrdered,
  ListX,
  Merge,
  type LucideIcon,
} from 'lucide-react';
import { api, posterUrl, type FileProblem, type Item, type LibraryHealth, type MissingEpisode } from '@/lib/api';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { kindIcon } from '@/lib/kinds';
import { Artwork } from '@/components/Artwork';
import { FixMatchDialog } from '@/components/FixMatch';
import { PlaybackReport } from '@/components/PlaybackReport';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { useLibraries } from '@/components/Sidebar';
import { Button } from '@/components/ui/button';
import { EmptyTrashButton } from '@/components/EmptyTrash';
import { MergeDialog } from '@/components/BulkEdit';

export function HealthPage() {
  const { data: libraries } = useLibraries();
  const reports = useQueries({
    queries: (libraries ?? []).map((lib) => ({
      queryKey: ['health', lib.id],
      queryFn: () => api.libraryHealth(lib.id),
    })),
  });
  const [identifying, setIdentifying] = useState<Item | null>(null);

  const loaded = libraries && reports.every((r) => r.data);
  const error = reports.find((r) => r.error)?.error;
  const healthy = loaded && reports.every((r) => problemCount(r.data!) === 0);

  return (
    <Page className="pb-16">
      <PageHeader title="Library Health" subtitle="Anything in your libraries worth a look, and how they play on different devices. Arc never changes your files to fix things." />
      <div className="space-y-12 px-6 md:px-10">
        {error && <ErrorState error={error} />}
        {!loaded && !error && <div className="h-64 animate-pulse rounded-2xl bg-surface" />}
        {healthy && (
          <EmptyState icon={<CheckCircle2 />} title="Everything looks good">
            Every file was read, recognised and identified, and nothing is missing.
          </EmptyState>
        )}
        {loaded &&
          !healthy &&
          reports.map((r) => <LibraryReport key={r.data!.library.id} report={r.data!} onIdentify={setIdentifying} />)}
        {libraries && libraries.length > 0 && <PlaybackReport libraries={libraries} />}
      </div>
      {identifying && <FixMatchDialog item={identifying} open onOpenChange={(o) => !o && setIdentifying(null)} />}
    </Page>
  );
}

function problemCount(r: LibraryHealth) {
  return (r.watch_problem ? 1 : 0) + r.unidentified.length + r.unreadable.length + r.unrecognised.length + r.missing_files.length + r.duplicates.length + r.missing_episodes.length;
}

function LibraryReport({ report: r, onIdentify }: { report: LibraryHealth; onIdentify: (item: Item) => void }) {
  const LibIcon = kindIcon(r.library.kind);
  if (problemCount(r) === 0) {
    return (
      <section className="flex items-center gap-3 text-sm text-muted">
        <CheckCircle2 className="size-5 text-accent" /> <span className="font-medium text-foreground">{r.library.name}</span> looks good.
      </section>
    );
  }
  const shows = r.library.kind === 'shows';
  return (
    <section>
      <h2 className="mb-5 flex items-center gap-2.5 text-xl font-semibold">
        <LibIcon className="size-5 text-muted" /> {r.library.name}
      </h2>
      <div className="space-y-8">
        {r.watch_problem && <p role="status" className="rounded-xl bg-surface p-4 text-sm text-warning">{r.watch_problem}</p>}
        <Category
          icon={HelpCircle}
          tone="warning"
          title="Not identified"
          hint={`Arc couldn't find these online, so they have no artwork or details. They're still playable.`}
          items={r.unidentified}
          render={(item) => <UnidentifiedRow key={item.id} item={item} onIdentify={() => onIdentify(item)} />}
        />
        <Category
          icon={FileWarning}
          tone="warning"
          title="Couldn't be read"
          hint="These files may be damaged or still downloading. Arc tries them again on every scan."
          items={r.unreadable}
          render={(f) => <FileRow key={f.file_id} file={f} detail={f.error} />}
        />
        <Category
          icon={FileQuestion}
          tone="warning"
          title={shows ? 'Not recognised as episodes' : 'Not recognised'}
          hint={
            shows
              ? 'Arc reads the season and episode from the file name, like “Show - S01E02”. Rename these and scan again to add them.'
              : 'Arc couldn’t tell what these files are.'
          }
          items={r.unrecognised}
          render={(f) => <FileRow key={f.file_id} file={f} />}
        />
        <Category
          icon={FileX2}
          title="Missing files"
          hint="These files weren't found in the last scan. If a drive is disconnected, reconnect it and scan. Arc keeps these titles and their history until the files are back, or until you remove them."
          items={r.missing_files}
          action={<EmptyTrashButton libraryId={r.library.id} libraryName={r.library.name} />}
          render={(f) => (
            <FileRow
              key={f.file_id}
              file={f}
              detail={[f.item_title, f.missing_since && `missing since ${fmt.dateTime(new Date(f.missing_since * 1000).toISOString())}`].filter(Boolean).join(' · ')}
            />
          )}
        />
        <Category
          icon={Copy}
          title="Duplicates"
          hint="Several copies of the same title. That can be on purpose, like a 4K and a 1080p version."
          items={r.duplicates}
          render={(d, i) =>
            d.kind === 'copies' ? (
              <DuplicateRow key={i} item={d.items[0]} lines={(d.files ?? []).map((f) => `${f.path} · ${fmt.fileSize(f.size)}`)} label={fmt.plural(d.files?.length ?? 0, 'copy', 'copies')} />
            ) : (
              <DuplicateRow key={i} item={d.items[0]} lines={d.items.map((it) => `${it.title}${it.year ? ` (${it.year})` : ''}`)} label={`${d.items.length} separate ${d.items[0].kind === 'series' ? 'shows' : 'movies'} matched to the same title`} merge={d.items[0].kind === 'movie' ? d.items : undefined} />
            )
          }
        />
        {shows && (
          <Category
            icon={ListX}
            title="Missing episodes"
            hint="Episodes that have aired but aren't in your library, in the episode order you chose for each show. Specials aren't counted."
            items={r.missing_episodes.filter((g) => g.missing.length > 0)}
            render={(g) => <GapsRow key={g.series.id} gaps={g} />}
          />
        )}
        {shows && (
          <Category
            icon={ListOrdered}
            title="Not in the chosen episode order"
            hint="You have these episodes, but the episode order picked for the show doesn't include them. They're listed separately on the show's page so nothing goes missing."
            items={r.missing_episodes.filter((g) => g.unplaced > 0)}
            render={(g) => <UnplacedRow key={g.series.id} gaps={g} />}
          />
        )}
      </div>
    </section>
  );
}

function Category<T>({
  icon: Icon,
  tone,
  title,
  hint,
  items,
  render,
  action,
}: {
  icon: LucideIcon;
  tone?: 'warning';
  title: string;
  hint: string;
  items: T[];
  render: (item: T, i: number) => ReactNode;
  /** A button for the whole category, e.g. Remove All. */
  action?: ReactNode;
}) {
  const [expanded, setExpanded] = useState(false);
  if (items.length === 0) return null;
  const limit = 5;
  const shown = expanded ? items : items.slice(0, limit);
  return (
    <div>
      <div className="mb-3 flex items-start gap-3">
        <div className={cn('mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-xl', tone === 'warning' ? 'bg-warning/12 text-warning' : 'bg-surface-2 text-muted')}>
          <Icon className="size-4" />
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="font-semibold">
            {title} <span className="ml-1 font-normal text-subtle">{items.length}</span>
          </h3>
          <p className="mt-0.5 max-w-3xl text-sm text-muted">{hint}</p>
        </div>
        {action}
      </div>
      <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
        {shown.map(render)}
        {items.length > limit && (
          <button onClick={() => setExpanded((e) => !e)} className="w-full px-4 py-3 text-left text-sm font-medium text-accent hover:bg-surface-2">
            {expanded ? 'Show fewer' : `Show all ${items.length}`}
          </button>
        )}
      </div>
    </div>
  );
}

/** Removes everything in a library whose files are gone, after asking. */

function UnidentifiedRow({ item, onIdentify }: { item: Item; onIdentify: () => void }) {
  if (item.kind === 'episode') {
    return (
      <Link to={`/series/${item.parent_id}`} className="flex items-center gap-4 px-4 py-3.5 transition-colors hover:bg-surface-2">
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">
            {item.title}
            {item.series_title && <span className="font-normal text-muted"> · {item.series_title}</span>}
          </p>
          <p className="truncate text-xs text-subtle">The file's episode number isn't in the show's episode list. It's still playable on the show's page.</p>
        </div>
        <ChevronRight className="size-4 text-subtle" />
      </Link>
    );
  }
  return (
    <div className="flex items-center gap-4 px-4 py-3">
      <Link to={item.kind === 'series' ? `/series/${item.id}` : `/movie/${item.id}`} className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium hover:underline">
          {item.title}
          {item.year ? <span className="font-normal text-muted"> ({item.year})</span> : null}
        </p>
        <p className="truncate text-xs text-subtle">{item.kind === 'series' ? 'Show · no match on TheTVDB or TMDB' : 'Movie · no match on TMDB'}</p>
      </Link>
      <Button size="sm" variant="secondary" onClick={onIdentify}>
        Identify…
      </Button>
    </div>
  );
}

function fileName(path: string) {
  return path.split(/[\\/]/).pop() ?? path;
}

function FileRow({ file, detail }: { file: FileProblem; detail?: string }) {
  const href = file.item_kind === 'movie' ? `/movie/${file.item_id}` : file.item_kind === 'episode' ? `/series/${file.series_id}` : undefined;
  const body = (
    <>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{fileName(file.path)}</p>
        {detail && <p className="truncate text-xs text-muted">{detail}</p>}
        <p className="truncate font-mono text-[11px] text-subtle" title={file.path}>
          {file.path}
        </p>
      </div>
      {href && <ChevronRight className="size-4 shrink-0 text-subtle" />}
    </>
  );
  return href ? (
    <Link to={href} className="flex items-center gap-4 px-4 py-3 transition-colors hover:bg-surface-2">
      {body}
    </Link>
  ) : (
    <div className="flex items-center gap-4 px-4 py-3">{body}</div>
  );
}

function DuplicateRow({ item, label, lines, merge }: { item: Item; label: string; lines: string[]; merge?: Item[] }) {
  const href = item.kind === 'series' ? `/series/${item.id}` : item.kind === 'episode' ? `/series/${item.parent_id}` : `/movie/${item.id}`;
  const [merging, setMerging] = useState(false);
  return (
    <div className="flex items-center transition-colors hover:bg-surface-2">
      <Link to={href} className="flex min-w-0 flex-1 items-center gap-4 px-4 py-3">
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">
            {item.title} <span className="font-normal text-muted">· {label}</span>
          </p>
          {lines.map((l) => (
            <p key={l} className="truncate font-mono text-[11px] text-subtle" title={l}>
              {l}
            </p>
          ))}
        </div>
        <ChevronRight className="size-4 shrink-0 text-subtle" />
      </Link>
      {merge && (
        <>
          <Button variant="secondary" size="sm" className="mr-4 shrink-0" onClick={() => setMerging(true)}>
            <Merge /> Merge
          </Button>
          <MergeDialog items={merge} open={merging} onOpenChange={setMerging} />
        </>
      )}
    </div>
  );
}

type Gaps = LibraryHealth['missing_episodes'][number];

function SeriesRow({ gaps, children }: { gaps: Gaps; children: ReactNode }) {
  return (
    <Link to={`/series/${gaps.series.id}`} className="flex items-center gap-4 px-4 py-3 transition-colors hover:bg-surface-2">
      <Artwork src={posterUrl(gaps.series)} alt="" icon="tv" className="w-10 shrink-0 rounded-md" />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{gaps.series.title}</p>
        {children}
      </div>
      <ChevronRight className="size-4 shrink-0 text-subtle" />
    </Link>
  );
}

function GapsRow({ gaps }: { gaps: Gaps }) {
  return (
    <SeriesRow gaps={gaps}>
      <p className="text-xs text-muted">
        {fmt.plural(gaps.missing.length, 'episode')} missing: <span className="text-foreground/80">{episodeRanges(gaps.missing)}</span>
      </p>
      <p className="text-[11px] text-subtle">
        In {gaps.ordering} · {fmt.plural(gaps.have, 'episode')} in your library
      </p>
    </SeriesRow>
  );
}

function UnplacedRow({ gaps }: { gaps: Gaps }) {
  return (
    <SeriesRow gaps={gaps}>
      <p className="text-xs text-muted">
        {fmt.plural(gaps.unplaced, 'episode')} {gaps.unplaced === 1 ? "isn't" : "aren't"} in {gaps.ordering}
      </p>
    </SeriesRow>
  );
}

/** "S1 E5, S2 E1–4, S3 E2" from a sorted list of episodes. */
export function episodeRanges(eps: MissingEpisode[]): string {
  const parts: string[] = [];
  let i = 0;
  while (i < eps.length) {
    let j = i;
    while (j + 1 < eps.length && eps[j + 1].season_number === eps[i].season_number && eps[j + 1].episode_number === eps[j].episode_number + 1) j++;
    const s = eps[i].season_number;
    parts.push(j > i ? `S${s} E${eps[i].episode_number}–${eps[j].episode_number}` : `S${s} E${eps[i].episode_number}`);
    i = j + 1;
  }
  return parts.join(', ');
}
