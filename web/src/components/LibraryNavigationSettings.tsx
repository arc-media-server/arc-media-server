import { ArrowDown, ArrowUp } from 'lucide-react';
import { useLibraries } from './Sidebar';
import type { useUpdatePrefs } from '@/lib/home';
import type { Prefs } from '@/lib/api';
import { navigationLibraries } from '@/lib/library-navigation';
import { Button } from './ui/button';
import { ErrorNotice } from './Page';

export function LibraryNavigationSettings({ prefs, update }: { prefs: Prefs; update: ReturnType<typeof useUpdatePrefs> }) {
  const libraries = useLibraries();
  const ordered = navigationLibraries(libraries.data ?? [], prefs.library_order);
  const changingOrder = update.variables?.library_order !== undefined;
  const move = (index: number, delta: number) => {
    const ids = ordered.map(l => l.id);
    [ids[index], ids[index + delta]] = [ids[index + delta], ids[index]];
    update.mutate({ library_order: ids });
  };
  return (
    <section aria-label="Library navigation order" className="space-y-3 rounded-2xl bg-surface p-4 ring-1 ring-border">
      <h2 className="font-medium">Library order</h2>
      <p className="text-sm text-muted">The order of libraries in your sidebar and menus.</p>
      {libraries.error ? <ErrorNotice error={libraries.error} onRetry={() => void libraries.refetch()} />
        : !libraries.data ? <p role="status" className="text-sm text-muted">Loading libraries…</p>
        : ordered.length === 0 ? <p className="text-sm text-muted">No libraries available.</p>
        : <ol className="space-y-2">{ordered.map((lib, index) => <li key={lib.id} className="flex items-center gap-2">
          <span className="min-w-0 flex-1 break-words text-sm">{lib.name}</span>
          <Button variant="ghost" size="icon" aria-label={`Move ${lib.name} up`} disabled={index === 0 || update.isPending} onClick={() => move(index, -1)}><ArrowUp /></Button>
          <Button variant="ghost" size="icon" aria-label={`Move ${lib.name} down`} disabled={index === ordered.length - 1 || update.isPending} onClick={() => move(index, 1)}><ArrowDown /></Button>
        </li>)}</ol>}
      {changingOrder && update.error && <ErrorNotice error={update.error} onRetry={() => update.variables && update.mutate(update.variables)} busy={update.isPending} />}
      {changingOrder && update.isSuccess && <p role="status" className="text-sm text-muted">Saved</p>}
      {!!prefs.library_order?.length && <Button variant="secondary" size="sm" disabled={update.isPending} onClick={() => update.mutate({ library_order: [] })}>Reset library order</Button>}
    </section>
  );
}
