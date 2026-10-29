import { useTranslation } from 'react-i18next';
import { useRef, useState } from 'react';
import { ArrowDown, ArrowUp, GripVertical } from 'lucide-react';
import { useLibraries } from './Sidebar';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import { navigationLibraries } from '@/lib/library-navigation';
import { Button } from './ui/button';
import { Dialog, DialogContent } from './ui/dialog';
import { ErrorNotice } from './Page';

/** The same per-profile navigation preference, edited where the libraries live. */
export function LibraryNavigationSettings() {
  const { t } = useTranslation('profileSettings');
  const libraries = useLibraries();
  const prefs = usePrefs();
  const update = useUpdatePrefs();
  const [draft, setDraft] = useState<number[] | null>(null);
  const dragged = useRef<number | null>(null);
  const ordered = navigationLibraries(libraries.data ?? [], draft ?? prefs.data?.library_order);
  const move = (from: number, to: number) => {
    const ids = ordered.map(l => l.id);
    ids.splice(to, 0, ids.splice(from, 1)[0]);
    setDraft(ids);
  };
  return <>
    <Button variant="ghost" size="icon" aria-label={t('libraryNavigationSettings.libraryOrder')} disabled={!prefs.data || !libraries.data?.length} onClick={() => { update.reset(); setDraft(prefs.data?.library_order ?? []); }}><GripVertical className="size-4" /></Button>
    <Dialog open={draft !== null} onOpenChange={open => { if (!open && !update.isPending) setDraft(null); }}>
      <DialogContent title={t('libraryNavigationSettings.libraryOrder')} description={t('libraryNavigationSettings.theOrderOfLibraries')} footer={<>
        <Button variant="ghost" disabled={update.isPending} onClick={() => setDraft(null)}>{t('libraryNavigationSettings.cancel')}</Button>
        <Button variant="primary" disabled={update.isPending} onClick={() => update.mutate({ library_order: draft ?? [] }, { onSuccess: () => setDraft(null) })}>{t('libraryNavigationSettings.save')}</Button>
      </>}>
        <p className="text-sm text-muted">{t('libraryNavigationSettings.dragHint')}</p>
        <ol className="space-y-2">{ordered.map((lib, index) => <li key={lib.id} draggable={!update.isPending} onDragStart={() => { dragged.current = index; }} onDragEnd={() => { dragged.current = null; }} onDragOver={e => e.preventDefault()} onDrop={e => { e.preventDefault(); if (dragged.current !== null && !update.isPending) move(dragged.current, index); dragged.current = null; }} className="flex items-center gap-2 rounded-xl bg-surface-2 p-2">
          <GripVertical className="size-4 shrink-0 text-subtle" aria-hidden />
          <span className="min-w-0 flex-1 break-words text-sm">{lib.name}</span>
          <Button variant="ghost" size="icon" aria-label={t('libraryNavigationSettings.moveUp', { name: lib.name })} disabled={index === 0 || update.isPending} onClick={() => move(index, index - 1)}><ArrowUp /></Button>
          <Button variant="ghost" size="icon" aria-label={t('libraryNavigationSettings.moveDown', { name: lib.name })} disabled={index === ordered.length - 1 || update.isPending} onClick={() => move(index, index + 1)}><ArrowDown /></Button>
        </li>)}</ol>
        <Button variant="secondary" size="sm" disabled={update.isPending} onClick={() => setDraft([])}>{t('libraryNavigationSettings.resetLibraryOrder')}</Button>
        <ErrorNotice error={update.error} />
      </DialogContent>
    </Dialog>
  </>;
}
