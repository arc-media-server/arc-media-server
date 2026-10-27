import { useTranslation } from 'react-i18next';
import type { ReactNode } from 'react';
import { Dialog as D } from 'radix-ui';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';

export const Dialog = D.Root;
export const DialogTrigger = D.Trigger;
export const DialogClose = D.Close;

/** A centred panel with a title, scrolling body and a footer for actions. */
export function DialogContent({
  title,
  description,
  footer,
  children,
  className,
  overlayClassName,
}: {
  title: string;
  description?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  className?: string;
  overlayClassName?: string;
}) {
  const { t } = useTranslation('common');
  return (
    <D.Portal>
      <D.Overlay className={cn("animate-fade-in fixed inset-0 z-50 bg-black/50 backdrop-blur-sm", overlayClassName)} />
      <D.Content
        className={cn(
          'animate-dialog-in glass fixed left-1/2 top-1/2 z-50 flex max-h-[min(860px,calc(100dvh-32px))] w-[min(560px,calc(100vw-32px))] -translate-x-1/2 -translate-y-1/2 flex-col rounded-3xl border border-border shadow-2xl shadow-black/50',
          className,
        )}
      >
        <div className="flex items-start gap-4 px-6 pb-2 pt-6">
          <div className="min-w-0 flex-1">
            <D.Title className="text-lg font-semibold">{title}</D.Title>
            {description ? (
              <D.Description className="mt-1 text-sm text-muted">{description}</D.Description>
            ) : (
              <D.Description className="sr-only">{title}</D.Description>
            )}
          </div>
          <D.Close className="-mr-2 -mt-1 rounded-full p-2 text-subtle transition-colors hover:bg-surface-3 hover:text-foreground" aria-label={t('actions.close')}>
            <X className="size-4" />
          </D.Close>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-3">{children}</div>
        {footer && <div className="flex items-center justify-end gap-2 border-t border-border px-6 py-4">{footer}</div>}
      </D.Content>
    </D.Portal>
  );
}
