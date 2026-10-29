import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Reorder, useDragControls } from 'motion/react';
import { ArrowUp, ArrowDown, GripVertical } from 'lucide-react';

/** Shared pointer/touch drag and keyboard controls for Home and Recommended editors. */
export function ArrangeRow<T>({ value, title, onMoveUp, onMoveDown, children }: { value: T; title: string; onMoveUp?: () => void; onMoveDown?: () => void; children: ReactNode }) {
  const { t } = useTranslation('home');
  const drag = useDragControls();
  return <Reorder.Item value={value} dragListener={false} dragControls={drag} className="flex items-center gap-2 rounded-2xl bg-surface-2/70 py-2 pl-1.5 pr-2 ring-1 ring-border" whileDrag={{ scale: 1.02, boxShadow: '0 12px 30px rgba(0,0,0,0.35)' }}>
    <button type="button" aria-label={t('rowActions.drag', { title })} onPointerDown={e => drag.start(e)} className="cursor-grab touch-none rounded-lg p-1.5 text-subtle hover:text-foreground active:cursor-grabbing"><GripVertical className="size-4" /></button>
    <div className="flex shrink-0 flex-col">
      <button type="button" aria-label={t('rowActions.moveUp', { title })} disabled={!onMoveUp} onClick={onMoveUp} className="rounded p-1 text-muted hover:bg-surface-3 disabled:opacity-25"><ArrowUp className="size-4" /></button>
      <button type="button" aria-label={t('rowActions.moveDown', { title })} disabled={!onMoveDown} onClick={onMoveDown} className="rounded p-1 text-muted hover:bg-surface-3 disabled:opacity-25"><ArrowDown className="size-4" /></button>
    </div>
    {children}
  </Reorder.Item>;
}
