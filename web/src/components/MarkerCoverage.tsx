import { useTranslation } from 'react-i18next';
import { useQueries } from '@tanstack/react-query';
import { Link } from 'react-router';
import { SkipForward } from 'lucide-react';
import { api, type Library, type MarkerCoverage } from '@/lib/api';
import { ErrorNotice } from '@/components/Page';

/** Which shows and movies have Skip Intro / Skip Credits markers, per library that offers them. */
export function MarkerCoverageReport({ libraries }: { libraries: Library[] }) {
  const { t } = useTranslation('libraryManagement');
  const offering = libraries.filter((l) => l.kind !== 'videos' && (l.skip_intros || l.skip_credits));
  const reports = useQueries({
    queries: offering.map((lib) => ({ queryKey: ['marker-coverage', lib.id], queryFn: () => api.markerCoverage(lib.id) })),
  });
  if (offering.length === 0) return null;
  return (
    <section aria-label={t("libraryManagement:markerCoverage.skipIntroAndCredits")}>
      <h2 className="mb-1 flex items-center gap-2.5 text-xl font-semibold">
        <SkipForward className="size-5 text-muted" /> {t("libraryManagement:markerCoverage.skipIntroCredits")}</h2>
      <p className="mb-4 max-w-3xl text-sm text-muted">{t("libraryManagement:markerCoverage.whichTitlesHaveIntro")}</p>
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
  const { t } = useTranslation('libraryManagement');
  const sum = (k: 'files' | 'intros' | 'credits' | 'checked') => titles.reduce((n, tabId) => n + tabId[k], 0);
  const files = sum('files');
  if (files === 0) return <p className="text-sm text-muted">{t("libraryManagement:markerCoverage.noFilesYet")}</p>;
  const unchecked = files - sum('checked');
  const unit = titles.some((tabId) => tabId.kind === 'series') ? 'episodes' : 'movies';
  return (
    <div className="rounded-2xl bg-surface ring-1 ring-border">
      <p className="px-4 py-3 text-sm">
        {[intros && (unit === 'episodes' ? t('libraryManagement:markerCoverage.episodeIntros', { count: sum('intros'), total: files }) : t('libraryManagement:markerCoverage.movieIntros', { count: sum('intros'), total: files })), credits && t('libraryManagement:markerCoverage.creditsCount', { count: sum('credits'), total: files }), unchecked > 0 && t('libraryManagement:markerCoverage.unchecked', { count: unchecked })]
          .filter(Boolean)
          .join(' · ')}
      </p>
      <details className="border-t border-border">
        <summary className="cursor-pointer px-4 py-2.5 text-sm font-medium text-accent">{t("libraryManagement:markerCoverage.byTitle")}</summary>
        <ul className="divide-y divide-border">
          {titles.map((tabId) => (
            <li key={tabId.id} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-4 py-2 text-sm">
              <Link to={`/${tabId.kind === 'series' ? 'series' : 'movie'}/${tabId.id}`} className="min-w-0 truncate hover:underline">
                {tabId.title}
              </Link>
              <span className="text-xs text-muted">
                {[
                  intros && tabId.kind === 'series' && t('libraryManagement:markerCoverage.introRatio', { found: tabId.intros, total: tabId.files }),
                  credits && (tabId.kind === 'series' ? t('libraryManagement:markerCoverage.creditsRatio', { found: tabId.credits, total: tabId.files }) : tabId.credits > 0 ? t('libraryManagement:markerCoverage.credits') : t('libraryManagement:markerCoverage.noCredits')),
                  tabId.files > tabId.checked && t('libraryManagement:markerCoverage.uncheckedTitle', { count: tabId.files - tabId.checked }),
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
