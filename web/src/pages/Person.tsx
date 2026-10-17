import { useParams } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { api, type PersonCredit } from '@/lib/api';
import * as fmt from '@/lib/format';
import { ErrorState, Page } from '@/components/Page';
import { PosterCard } from '@/components/PosterCard';

const roleNames = { actor: 'Acting', director: 'Directing', writer: 'Writing', creator: 'Created' } as const;

/** Someone in the cast or crew, and everything they're in that's in the libraries. */
export function PersonPage() {
  const { id } = useParams();
  const { data, error } = useQuery({ queryKey: ['person', id], queryFn: () => api.person(Number(id)) });
  if (error) return <ErrorState error={error} />;
  if (!data) return <div className="h-64 animate-pulse bg-surface" />;
  const p = data.person;
  const acting = data.credits.filter((c) => c.roles.includes('actor'));
  const crew = data.credits.filter((c) => c.roles.some((r) => r !== 'actor'));

  return (
    <Page className="pb-16">
      <div className="flex flex-wrap items-end gap-6 px-6 pb-10 pt-14 md:px-10">
        <div className="size-36 shrink-0 overflow-hidden rounded-full bg-surface-2 shadow-2xl ring-1 ring-border">
          {p.image ? (
            <img src={p.image} alt="" className="size-full object-cover" />
          ) : (
            <span className="flex size-full items-center justify-center text-5xl font-semibold text-subtle">{p.name.charAt(0)}</span>
          )}
        </div>
        <div>
          <h1 className="text-4xl font-bold leading-tight">{p.name}</h1>
          <p className="mt-1 text-sm text-muted">{fmt.plural(p.count, 'title')} in your libraries</p>
        </div>
      </div>
      <div className="space-y-12 px-6 md:px-10">
        {acting.length > 0 && <Credits title="Acting" credits={acting} />}
        {crew.length > 0 && <Credits title="Behind the camera" credits={crew} />}
      </div>
    </Page>
  );
}

function Credits({ title, credits }: { title: string; credits: PersonCredit[] }) {
  return (
    <section>
      <h2 className="mb-4 text-lg font-semibold">{title}</h2>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(132px,1fr))] gap-x-5 gap-y-7 sm:grid-cols-[repeat(auto-fill,minmax(156px,1fr))]">
        {credits.map((c) => (
          <div key={c.item.id}>
            <PosterCard item={c.item} />
            <p className="mt-0.5 truncate px-0.5 text-xs text-muted">
              {title === 'Acting'
                ? c.character || 'Cast'
                : c.roles
                    .filter((r) => r !== 'actor')
                    .map((r) => roleNames[r])
                    .join(', ')}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}
