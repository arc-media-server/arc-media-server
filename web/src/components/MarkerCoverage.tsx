import { useQueries } from '@tanstack/react-query';
import { Link } from 'react-router';
import { SkipForward } from 'lucide-react';
import { api, type Library, type MarkerCoverage } from '@/lib/api';
import { ErrorNotice } from '@/components/Page';

/** Which shows and movies have Skip Intro / Skip Credits markers, per library that offers them. */
export function MarkerCoverageReport({ libraries }: { libraries: Library[] }) {
  const offering = libraries.filter((l) => l.kind !== 'videos' && (l.skip_intros || l.skip_credits));
  const reports = useQueries({
    queries: offering.map((lib) => ({ queryKey: ['marker-coverage', lib.id], queryFn: () => api.markerCoverage(lib.id) })),
  });
  if (offering.length === 0) return null;
  return (
    <section aria-label="Skip Intro and Credits">
      <h2 className="mb-1 flex items-center gap-2.5 text-xl font-semibold">
        <SkipForward className="size-5 text-muted" /> Skip Intro & Credits
      </h2>
      <p className="mb-4 max-w-3xl text-sm text-muted">
        Which titles have intro and credits markers, so you know where Skip Intro and Skip Credits will appear. New files are
        checked soon after they&apos;re added; comparing episodes on this server follows Maintenance.
      </p>
      <div className="space-y-6">
        {offering.map((lib, i) => {
          const q = reports[i];
          return (
            <div key={lib.id}>
              <h3 className="mb-2 font-medium">{lib.name}</h3>
              <ErrorNotice error={q.error} onRetry={() => void q.refetch()} />
              {!q.data && !q.error && <div className="h-16 animate-pulse rounded-xl bg-surface" />}
              {q.data && <LibraryCoverage titles={q.data.titles} intros={q.data.skip_intros && lib.kind === 'shows'} credits={q.data.skip_credits} />}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function LibraryCoverage({ titles, intros, credits }: { titles: MarkerCoverage[]; intros: boolean; credits: boolean }) {
  const sum = (k: 'files' | 'intros' | 'credits' | 'checked') => titles.reduce((n, t) => n + t[k], 0);
  const files = sum('files');
  if (files === 0) return <p className="text-sm text-muted">No files yet.</p>;
  const unchecked = files - sum('checked');
  const unit = titles.some((t) => t.kind === 'series') ? 'episodes' : 'movies';
  return (
    <div className="rounded-2xl bg-surface ring-1 ring-border">
      <p className="px-4 py-3 text-sm">
        {[intros && `${sum('intros')} of ${files} ${unit} have an intro`, credits && `${sum('credits')} of ${files} have credits`, unchecked > 0 && `${unchecked} not checked yet`]
          .filter(Boolean)
          .join(' · ')}
      </p>
      <details className="border-t border-border">
        <summary className="cursor-pointer px-4 py-2.5 text-sm font-medium text-accent">By title</summary>
        <ul className="divide-y divide-border">
          {titles.map((t) => (
            <li key={t.id} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-4 py-2 text-sm">
              <Link to={`/${t.kind === 'series' ? 'series' : 'movie'}/${t.id}`} className="min-w-0 truncate hover:underline">
                {t.title}
              </Link>
              <span className="text-xs text-muted">
                {[
                  intros && t.kind === 'series' && `Intro ${t.intros}/${t.files}`,
                  credits && (t.kind === 'series' ? `Credits ${t.credits}/${t.files}` : t.credits > 0 ? 'Credits' : 'No credits'),
                  t.files > t.checked && `${t.files - t.checked} not checked`,
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </li>
          ))}
        </ul>
      </details>
    </div>
  );
}
