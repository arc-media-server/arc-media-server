import { Link } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { CheckCircle2, ChevronRight, Film, HelpCircle, Tv } from 'lucide-react';
import { api, type Item, type Library } from '@/lib/api';
import { EmptyState, Page, PageHeader } from '@/components/Page';
import { useLibraries } from '@/components/Sidebar';

export function AttentionPage() {
  const { data: libraries } = useLibraries();
  const results = useQuery({
    queryKey: ['attention-detail', libraries?.map((l) => l.id)],
    enabled: !!libraries,
    queryFn: async () => Promise.all(libraries!.map(async (lib) => ({ lib, items: await api.unmatched(lib.id) }))),
  });

  const total = results.data?.reduce((n, r) => n + r.items.length, 0) ?? 0;

  return (
    <Page className="pb-16">
      <PageHeader title="Needs Attention" subtitle="Things Arc couldn't identify online. They're still playable." />
      <div className="space-y-10 px-6 md:px-10">
        {results.data && total === 0 && (
          <EmptyState icon={<CheckCircle2 />} title="All good">
            Everything in your libraries has been identified.
          </EmptyState>
        )}
        {results.data
          ?.filter((r) => r.items.length)
          .map(({ lib, items }) => (
            <LibrarySection key={lib.id} lib={lib} items={items} />
          ))}
      </div>
    </Page>
  );
}

function LibrarySection({ lib, items }: { lib: Library; items: Item[] }) {
  return (
    <section>
      <h2 className="mb-3 text-lg font-semibold">{lib.name}</h2>
      <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
        {items.map((item) => (
          <Row key={item.id} item={item} />
        ))}
      </div>
    </section>
  );
}

function Row({ item }: { item: Item }) {
  const href = item.kind === 'movie' ? `/movie/${item.id}` : `/series/${item.kind === 'episode' ? item.parent_id : item.id}`;
  const Icon = item.kind === 'movie' ? Film : item.kind === 'series' ? Tv : HelpCircle;
  const reason =
    item.kind === 'episode'
      ? "This file's episode number isn't in the show's episode list"
      : `No ${item.kind === 'series' ? 'show' : 'movie'} called “${item.title}”${item.year ? ` (${item.year})` : ''} was found`;
  return (
    <Link to={href} className="flex items-center gap-4 px-4 py-3.5 transition-colors hover:bg-surface-2">
      <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-warning/12 text-warning">
        <Icon className="size-[18px]" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{item.title}</p>
        <p className="truncate text-xs text-subtle">{reason}</p>
      </div>
      <ChevronRight className="size-4 text-subtle" />
    </Link>
  );
}
