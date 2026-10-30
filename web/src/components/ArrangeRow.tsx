import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Reorder, useDragControls } from 'motion/react';
import { ArrowUp, ArrowDown, GripVertical } from 'lucide-react';
import { cn } from '@/lib/utils';

/** Shared pointer/touch drag and keyboard controls for Home, Recommended and library order. */
export function ArrangeRow<T>({ value, title, onMoveUp, onMoveDown, dense = false, children }: { value: T; title: string; onMoveUp?: () => void; onMoveDown?: () => void; dense?: boolean; children: ReactNode }) {
  const { t } = useTranslation('home');
  const drag = useDragControls();
  return <Reorder.Item value={value} dragListener={false} dragControls={drag} className={cn('flex items-center ring-1 ring-border', dense ? 'gap-1.5 rounded-xl bg-surface-2 py-1 pl-0.5 pr-2.5' : 'gap-2 rounded-2xl bg-surface-2/70 py-2 pl-1.5 pr-2')} whileDrag={{ scale: 1.02, boxShadow: '0 12px 30px rgba(0,0,0,0.35)' }}>
    <button type="button" aria-label={t('rowActions.drag', { title })} onPointerDown={e => drag.start(e)} className="cursor-grab touch-none rounded-lg p-1.5 text-subtle hover:text-foreground active:cursor-grabbing"><GripVertical className="size-4" /></button>
    <div className="flex shrink-0 flex-col">
      <button type="button" aria-label={t('rowActions.moveUp', { title })} disabled={!onMoveUp} onClick={onMoveUp} className={cn('rounded text-muted hover:bg-surface-3 disabled:opacity-25', dense ? 'p-0.5' : 'p-1')}><ArrowUp className={dense ? 'size-3.5' : 'size-4'} /></button>
      <button type="button" aria-label={t('rowActions.moveDown', { title })} disabled={!onMoveDown} onClick={onMoveDown} className={cn('rounded text-muted hover:bg-surface-3 disabled:opacity-25', dense ? 'p-0.5' : 'p-1')}><ArrowDown className={dense ? 'size-3.5' : 'size-4'} /></button>
    </div>
    {children}
  </Reorder.Item>;
}

/**
 * A reorderable list: drag (pointer and touch) through each row's handle, or the Move up/down
 * buttons from the keyboard, which announce the new position to screen readers.
 */
export function ReorderList<T>({ items, keyOf, titleOf, onReorder, disabled = false, dense = false, className, label, children }: {
  items: T[];
  keyOf: (item: T) => string | number;
  titleOf: (item: T) => string;
  onReorder: (items: T[]) => void;
  disabled?: boolean;
  dense?: boolean;
  className?: string;
  /** Accessible name for the list. */
  label?: string;
  children: (item: T, index: number) => ReactNode;
}) {
  const { t } = useTranslation('home');
  const [announcement, announce] = useState('');
  const move = (from: number, delta: number) => {
    const to = from + delta;
    if (disabled || to < 0 || to >= items.length) return;
    const next = [...items];
    [next[from], next[to]] = [next[to], next[from]];
    onReorder(next);
    announce(t('editor.position', { title: titleOf(next[to]), position: to + 1, total: next.length }));
  };
  return <>
    <p className="sr-only" role="status" aria-live="polite">{announcement}</p>
    <Reorder.Group axis="y" values={items} onReorder={next => { if (!disabled) onReorder(next); }} aria-label={label} className={className}>
      {items.map((item, index) => (
        <ArrangeRow key={keyOf(item)} value={item} title={titleOf(item)} dense={dense}
          onMoveUp={index > 0 && !disabled ? () => move(index, -1) : undefined}
          onMoveDown={index < items.length - 1 && !disabled ? () => move(index, 1) : undefined}>
          {children(item, index)}
        </ArrangeRow>
      ))}
    </Reorder.Group>
  </>;
}
