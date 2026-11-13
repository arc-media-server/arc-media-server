import { useState, type ComponentProps, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, type Item } from '@/lib/api';
import { useIsAdmin } from '@/lib/auth';
import { useLibraries } from './Sidebar';
import { EditDetailsDialog } from './EditDetails';
import { ErrorNotice } from './Page';
import type { PosterCard } from './PosterCard';

type Actions = Pick<ComponentProps<typeof PosterCard>, 'hover'>;

/** Shared per-title Home actions; bulk selection belongs in the library view. */
export function HomeRowActions({ children }: { children: (props: (item: Item) => Actions) => ReactNode }) {
  const admin = useIsAdmin();
  const libraries = useLibraries();
  const [editing, setEditing] = useState<string>();
  const item = useQuery({ queryKey: ['item', editing], queryFn: () => api.item(editing!), enabled: !!editing });
  const cardProps = (i: Item): Actions => ({ hover: admin ? {
    onEdit: libraries.data?.find(l => l.id === i.library_id)?.kind === 'videos' ? undefined : () => setEditing(i.id),
  } : {} });
  return <>
    {children(cardProps)}
    {editing && item.error && <ErrorNotice error={item.error} onRetry={() => void item.refetch()} />}
    {editing && item.data && <EditDetailsDialog item={item.data} open onOpenChange={open => { if (!open) setEditing(undefined); }} />}
  </>;
}
