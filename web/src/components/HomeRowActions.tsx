import { createContext, useContext, useEffect, useId, useState, type ComponentProps, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { api, type Item } from '@/lib/api';
import { useIsAdmin } from '@/lib/auth';
import { useLibraries } from './Sidebar';
import { SelectionBar } from './BulkEdit';
import { EditDetailsDialog } from './EditDetails';
import { ErrorNotice } from './Page';
import type { PosterCard } from './PosterCard';

type Actions = Pick<ComponentProps<typeof PosterCard>, 'hover' | 'onSelect' | 'selected'>;

const ActiveRow = createContext<{ active?: string; choose: (id: string) => void }>({ choose: () => {} });
export function HomeActionsProvider({ children }: { children: ReactNode }) {
  const [active, choose] = useState<string>();
  return <ActiveRow.Provider value={{ active, choose }}>{children}</ActiveRow.Provider>;
}

/** One Home row's loaded titles, scoped to one library for bulk operations. */
export function HomeRowActions({ items, title, children }: { items: Item[]; title: string; children: (props: (item: Item) => Actions) => ReactNode }) {
  const { t } = useTranslation('home');
  const admin = useIsAdmin();
  const rowID = useId();
  const active = useContext(ActiveRow);
  const libraries = useLibraries();
  const [editing, setEditing] = useState<string>();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [libraryID, setLibraryID] = useState<number>();
  useEffect(() => { if (active.active !== rowID) { setLibraryID(undefined); setSelected(new Set()); } }, [active.active, rowID]);
  const library = libraries.data?.find(l => l.id === libraryID);
  const eligible = [...new Map(items.filter(i => i.library_id === libraryID && i.kind !== 'episode').map(i => [i.id, i])).values()];
  useEffect(() => {
    const escape = (e: KeyboardEvent) => { if (e.key === 'Escape') { setLibraryID(undefined); setSelected(new Set()); } };
    window.addEventListener('keydown', escape);
    return () => window.removeEventListener('keydown', escape);
  }, []);
  const item = useQuery({ queryKey: ['item', editing], queryFn: () => api.item(editing!), enabled: !!editing });
  const cardProps = (i: Item): Actions => {
    if (library && i.library_id === library.id && i.kind !== 'episode') return {
      selected: selected.has(i.id), onSelect: () => setSelected(old => { const next = new Set(old); if (next.has(i.id)) next.delete(i.id); else next.add(i.id); return next; }),
    };
    return { hover: admin ? {
      onEdit: libraries.data?.find(l => l.id === i.library_id)?.kind === 'videos' ? undefined : () => setEditing(i.id),
      onSelect: i.kind !== 'episode' && libraryID === undefined ? () => { active.choose(rowID); setLibraryID(i.library_id); setSelected(new Set([i.id])); } : undefined,
    } : {} };
  };
  return <>
    {children(cardProps)}
    {editing && item.error && <ErrorNotice error={item.error} onRetry={() => void item.refetch()} />}
    {editing && item.data && <EditDetailsDialog item={item.data} open onOpenChange={open => { if (!open) setEditing(undefined); }} />}
    {admin && library && <SelectionBar library={library} items={eligible} selected={selected}
      scope={t('rows.selectionScope', { row: title, library: library.name })}
      onSelectAll={() => setSelected(new Set(eligible.map(i => i.id)))} onClear={() => setSelected(new Set())}
      onDone={() => { setLibraryID(undefined); setSelected(new Set()); }} />}
  </>;
}
