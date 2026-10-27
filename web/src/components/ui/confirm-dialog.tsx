import { useTranslation } from 'react-i18next';
import type { ReactNode } from 'react';
import { AlertDialog } from 'radix-ui';
import { Loader2 } from 'lucide-react';
import { Button } from './button';

/**
 * A modal asking the user to confirm something consequential. The confirm button shows a spinner
 * while busy; errors are shown inside the dialog so it can be retried.
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  children,
  confirmLabel,
  destructive,
  busy,
  error,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  children: ReactNode;
  confirmLabel: string;
  destructive?: boolean;
  busy?: boolean;
  error?: string;
  onConfirm: () => void;
}) {
  const { t } = useTranslation('common');
  return (
    <AlertDialog.Root open={open} onOpenChange={(o) => !busy && onOpenChange(o)}>
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="animate-fade-in fixed inset-0 z-50 bg-black/50 backdrop-blur-sm" />
        <AlertDialog.Content className="animate-dialog-in glass fixed left-1/2 top-1/2 z-50 w-[min(440px,calc(100vw-32px))] -translate-x-1/2 -translate-y-1/2 rounded-3xl border border-border p-6 shadow-2xl shadow-black/50">
          <AlertDialog.Title className="text-lg font-semibold">{title}</AlertDialog.Title>
          <AlertDialog.Description asChild>
            <div className="mt-2 space-y-2 text-sm leading-relaxed text-muted">{children}</div>
          </AlertDialog.Description>
          {error && <p className="mt-4 rounded-xl bg-warning/10 px-3 py-2 text-sm text-warning">{error}</p>}
          <div className="mt-6 flex justify-end gap-2">
            <AlertDialog.Cancel asChild>
              <Button variant="ghost" disabled={busy}>{t('actions.cancel')}</Button>
            </AlertDialog.Cancel>
            <Button variant={destructive ? 'danger' : 'primary'} disabled={busy} onClick={onConfirm}>
              {busy && <Loader2 className="animate-spin" />} {confirmLabel}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}
