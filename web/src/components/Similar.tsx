import { useQuery } from '@tanstack/react-query';
import { api, type Item } from '@/lib/api';
import { MediaRow } from '@/components/MediaRow';

/** "More like this" on a movie or show's page (hidden when there's nothing). */
export function SimilarRow({ item }: { item: Item }) {
  const { data } = useQuery({ queryKey: ['similar', item.id], queryFn: () => api.similar(item.id) });
  return (
    <div className="mt-10">
      <MediaRow title="More Like This" items={data ?? []} />
    </div>
  );
}
