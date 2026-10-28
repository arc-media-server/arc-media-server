import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { Bookmark } from 'lucide-react';
import { api } from '@/lib/api';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';

/** The profile's watchlist: movies and shows saved for later, newest first. */
export function WatchlistPage() {
  const { t } = useTranslation('watchlist');
  const { data, error } = useQuery({ queryKey: ['watchlist'], queryFn: api.watchlist });
  if (error) return <ErrorState error={error} />;
  return (
    <Page className="pb-16">
      <PageHeader title={t('common:navigation.watchlist')} subtitle={t('description')} />
      <div className="px-6 md:px-10">
        {!data ? (
          <PosterGridSkeleton count={6} />
        ) : data.length === 0 ? (
          <EmptyState icon={<Bookmark />} title={t('empty.heading')}>{t('empty.description')}</EmptyState>
        ) : (
          <PosterGrid items={data} hover={{}} />
        )}
      </div>
    </Page>
  );
}
