import { useQuery } from '@tanstack/react-query';
import { Bookmark } from 'lucide-react';
import { api } from '@/lib/api';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';

/** The profile's watchlist: movies and shows saved for later, newest first. */
export function WatchlistPage() {
  const { data, error } = useQuery({ queryKey: ['watchlist'], queryFn: api.watchlist });
  if (error) return <ErrorState error={error} />;
  return (
    <Page className="pb-16">
      <PageHeader title="Watchlist" subtitle="Movies and shows you've saved for later. A movie leaves the list once you've watched it." />
      <div className="px-6 md:px-10">
        {!data ? (
          <PosterGridSkeleton count={6} />
        ) : data.length === 0 ? (
          <EmptyState icon={<Bookmark />} title="Nothing saved yet">
            Use the bookmark button on a movie or show to add it here.
          </EmptyState>
        ) : (
          <PosterGrid items={data} />
        )}
      </div>
    </Page>
  );
}
