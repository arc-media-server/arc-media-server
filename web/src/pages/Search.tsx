import { useSearchParams } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { SearchX } from 'lucide-react';
import { api } from '@/lib/api';
import { EmptyState, Page, PageHeader } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';

export function SearchPage() {
  const [params] = useSearchParams();
  const q = params.get('q')?.trim() ?? '';
  const results = useQuery({
    queryKey: ['search', q],
    enabled: q.length > 0,
    placeholderData: (prev) => prev,
    queryFn: async () => {
      const [movies, shows] = await Promise.all([
        api.items({ q, kind: 'movie', limit: 60 }),
        api.items({ q, kind: 'series', limit: 60 }),
      ]);
      return { movies: movies.items, shows: shows.items };
    },
  });

  const data = results.data;
  const none = data && data.movies.length === 0 && data.shows.length === 0;

  return (
    <Page className="pb-16">
      <PageHeader title={q ? `Results for “${q}”` : 'Search'} />
      <div className="space-y-12 px-6 md:px-10">
        {!q ? null : !data ? (
          <PosterGridSkeleton count={6} />
        ) : none ? (
          <EmptyState icon={<SearchX />} title="No results">
            Nothing in your libraries matches “{q}”.
          </EmptyState>
        ) : (
          <>
            {data.shows.length > 0 && (
              <section>
                <h2 className="mb-4 text-lg font-semibold">TV Shows</h2>
                <PosterGrid items={data.shows} />
              </section>
            )}
            {data.movies.length > 0 && (
              <section>
                <h2 className="mb-4 text-lg font-semibold">Movies</h2>
                <PosterGrid items={data.movies} />
              </section>
            )}
          </>
        )}
      </div>
    </Page>
  );
}
