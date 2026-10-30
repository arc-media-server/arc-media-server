import { useTranslation } from 'react-i18next';
import { createElement, useEffect, useRef, useState } from 'react';
import { ArrowDownUp, MoreHorizontal } from 'lucide-react';
import type { Library } from '@/lib/api';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import { kindIcon } from '@/lib/kinds';
import { navigationLibraries } from '@/lib/library-navigation';
import { useLibraries } from './Sidebar';
import { ReorderList } from './ArrangeRow';
import { ErrorNotice } from './Page';
import { Button } from './ui/button';
import { Dialog, DialogContent } from './ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ui/dropdown-menu';

/** A sidebar library's ⋯: quiet at rest, full strength on row hover, keyboard focus and while open. */
export const quietMenuTrigger = 'size-8 text-subtle/80 group-hover:text-muted group-focus-within:text-muted hover:text-foreground focus-visible:text-foreground data-[state=open]:bg-surface-2 data-[state=open]:text-foreground';

/**
 * Editing the profile's library order (`library_order`, a per-profile preference). Only the
 * libraries the server lists for this profile take part, and ones added later keep their server
 * order after the arranged ones.
 */
export function useLibraryReorder() {
  const libraries = useLibraries();
  const prefs = usePrefs();
  const update = useUpdatePrefs();
  const [draft, setDraft] = useState<number[] | null>(null);
  const all = libraries.data ?? [];
  const ordered = navigationLibraries(all, draft ?? prefs.data?.library_order);
  return {
    editing: draft !== null,
    ordered,
    pending: update.isPending,
    error: update.error,
    canReorder: !!prefs.data && all.length > 1,
    start: () => { update.reset(); setDraft(navigationLibraries(all, prefs.data?.library_order).map(l => l.id)); },
    cancel: () => { if (!update.isPending) { update.reset(); setDraft(null); } },
    setOrder: (libs: Library[]) => setDraft(libs.map(l => l.id)),
    reset: () => setDraft([]),
    save: () => {
      const ids = ordered.map(l => l.id);
      // The server's own order is saved as no preference, so it keeps following the server.
      const same = ids.join() === all.map(l => l.id).join();
      update.mutate({ library_order: same ? [] : ids }, { onSuccess: () => setDraft(null) });
    },
  };
}

export type LibraryReorder = ReturnType<typeof useLibraryReorder>;

function Rows({ reorder, dense }: { reorder: LibraryReorder; dense?: boolean }) {
  const { t } = useTranslation('common');
  return (
    <ReorderList items={reorder.ordered} keyOf={l => l.id} titleOf={l => l.name} onReorder={reorder.setOrder} disabled={reorder.pending} dense={dense}
      label={t('libraryOrder.title')} className={dense ? 'space-y-1' : 'space-y-1.5'}>
      {lib => <>
        <span className="shrink-0 text-muted [&_svg]:size-4">{createElement(kindIcon(lib.kind))}</span>
        <span className={dense ? 'min-w-0 flex-1 truncate text-[13.5px] font-medium' : 'min-w-0 flex-1 break-words text-sm font-medium'}>{lib.name}</span>
      </>}
    </ReorderList>
  );
}

function ResetButton({ reorder }: { reorder: LibraryReorder }) {
  const { t } = useTranslation('common');
  return (
    <button type="button" disabled={reorder.pending} onClick={reorder.reset}
      className="rounded text-xs text-subtle underline-offset-2 transition-colors hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-50">
      {t('libraryOrder.reset')}
    </button>
  );
}

/** The sidebar's library list while it's being arranged, in place. */
export function SidebarLibraryReorder({ reorder }: { reorder: LibraryReorder }) {
  const { t } = useTranslation('common');
  const root = useRef<HTMLDivElement>(null);
  const { cancel, pending } = reorder;
  // Start at the first row's controls (the ⋯ that opened this has gone), once the menu has closed.
  useEffect(() => {
    const frame = requestAnimationFrame(() => root.current?.querySelector<HTMLButtonElement>('li button')?.focus());
    return () => cancelAnimationFrame(frame);
  }, []);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || e.defaultPrevented || pending) return;
      e.preventDefault();
      cancel();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [cancel, pending]);
  return (
    <div ref={root}>
      <Rows reorder={reorder} dense />
      {reorder.error && <div className="mt-3"><ErrorNotice error={reorder.error} busy={pending} onRetry={reorder.save} /></div>}
      <div className="mt-3 flex gap-2">
        <Button size="sm" variant="ghost" className="flex-1" disabled={pending} onClick={cancel}>{t('actions.cancel')}</Button>
        <Button size="sm" variant="primary" className="flex-1" disabled={pending} onClick={reorder.save}>{t('actions.done')}</Button>
      </div>
      <div className="mt-2.5 text-center"><ResetButton reorder={reorder} /></div>
    </div>
  );
}

/** Phones have no sidebar, so the same list opens in a dialog from the profile menu. */
export function LibraryReorderDialog({ reorder }: { reorder: LibraryReorder }) {
  const { t } = useTranslation('common');
  return (
    <Dialog open={reorder.editing} onOpenChange={open => { if (!open) reorder.cancel(); }}>
      {reorder.editing && (
        <DialogContent title={t('libraryOrder.title')} description={t('libraryOrder.hint')} footer={<>
          <span className="mr-auto"><ResetButton reorder={reorder} /></span>
          <Button variant="ghost" disabled={reorder.pending} onClick={reorder.cancel}>{t('actions.cancel')}</Button>
          <Button variant="primary" disabled={reorder.pending} onClick={reorder.save}>{t('actions.done')}</Button>
        </>}>
          <Rows reorder={reorder} />
          {reorder.error && <div className="mt-3"><ErrorNotice error={reorder.error} busy={reorder.pending} onRetry={reorder.save} /></div>}
        </DialogContent>
      )}
    </Dialog>
  );
}

/** The menu item that starts arranging, shared by the library ⋯ and the phone profile menu. */
export function ReorderLibrariesItem({ onSelect }: { onSelect: () => void }) {
  const { t } = useTranslation('common');
  return <DropdownMenuItem onSelect={onSelect}><ArrowDownUp className="size-4 text-muted" />{' '}{t('libraryOrder.reorder')}</DropdownMenuItem>;
}

/** A profile without library management still gets a ⋯, holding just Reorder libraries. */
export function LibraryOrderMenu({ lib, onReorder }: { lib: Library; onReorder: () => void }) {
  const { t } = useTranslation('library');
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" data-library-menu={lib.id} aria-label={t('libraryActions', { name: lib.name })} className={quietMenuTrigger}><MoreHorizontal /></Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <ReorderLibrariesItem onSelect={onReorder} />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
