import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Trash2 } from 'lucide-react';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

export function EmptyTrashDialog({ libraryId, libraryName, open, onOpenChange }: {
  libraryId: number; libraryName: string; open: boolean; onOpenChange: (open: boolean) => void;
}) {
  const qc = useQueryClient();
  const empty = useMutation({ mutationFn: () => api.emptyTrash(libraryId), onSuccess: () => {
    onOpenChange(false);
    void qc.invalidateQueries();
  } });
  return <ConfirmDialog open={open} onOpenChange={o => {
    if (empty.isPending) return;
    onOpenChange(o);
    if (!o) empty.reset();
  }} title="Remove titles whose files are gone?" confirmLabel="Remove" destructive busy={empty.isPending}
    error={empty.error?.message} onConfirm={() => empty.mutate()}>
    <p>Arc forgets the missing files in {libraryName}, and any movie, episode or show left with no files at all, including their watch history. Titles that still have a file are kept.</p>
    <p>Only do this if the files are gone for good, not on a disconnected drive. A backup is made first. Media files are never deleted.</p>
  </ConfirmDialog>;
}

export function EmptyTrashButton({ libraryId, libraryName }: { libraryId: number; libraryName: string }) {
  const [open, setOpen] = useState(false);
  return <><Button variant="secondary" size="sm" className="shrink-0" onClick={() => setOpen(true)}><Trash2 /> Remove All…</Button>
    <EmptyTrashDialog libraryId={libraryId} libraryName={libraryName} open={open} onOpenChange={setOpen} /></>;
}
