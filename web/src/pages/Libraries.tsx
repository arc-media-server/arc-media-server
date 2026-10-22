import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { FolderPlus, Loader2, Pencil, Plus, Trash2 } from 'lucide-react';
import { kindIcon, kindUnit } from '@/lib/kinds';
import { api, type Library } from '@/lib/api';
import * as fmt from '@/lib/format';
import { LibraryEditorDialog } from '@/components/LibraryEditor';
import { EmptyState, ErrorState, Page, PageHeader } from '@/components/Page';
import { useLibraries } from '@/components/Sidebar';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

export function LibrariesPage() {
  const qc = useQueryClient();
  const query = useLibraries();
  const { data: libraries } = query;
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = useState<Library | 'new' | null>(null);
  const [removing, setRemoving] = useState<Library | null>(null);

  // /libraries?add=1 opens the add dialog straight away (from the welcome screen).
  useEffect(() => {
    if (params.get('add')) {
      setEditing('new');
      setParams({}, { replace: true });
    }
  }, [params, setParams]);

  const remove = useMutation({
    mutationFn: (lib: Library) => api.deleteLibrary(lib.id),
    onSuccess: () => {
      setRemoving(null);
      return qc.invalidateQueries();
    },
  });

  if (query.error && !libraries) return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
  return (
    <Page className="pb-16">
      <PageHeader
        title="Libraries"
        subtitle="The folders Arc looks in. New files show up on their own; Arc never changes them."
        actions={
          <Button variant="secondary" size="sm" onClick={() => setEditing('new')}>
            <Plus /> Add Library
          </Button>
        }
      />
      <div className="space-y-4 px-6 md:px-10">
        {libraries?.length === 0 && (
          <EmptyState icon={<FolderPlus />} title="No libraries yet">
            Add a folder of movies or TV shows to get started.
          </EmptyState>
        )}
        {libraries && libraries.length > 0 && (
          <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">
            {libraries.map((lib) => (
              <LibraryRow key={lib.id} lib={lib} onEdit={() => setEditing(lib)} onRemove={() => setRemoving(lib)} />
            ))}
          </div>
        )}
      </div>

      <LibraryEditorDialog
        library={editing && editing !== 'new' ? editing : undefined}
        open={!!editing}
        onOpenChange={(o) => !o && setEditing(null)}
      />
      <ConfirmDialog
        open={!!removing}
        onOpenChange={(o) => {
          if (!o) {
            setRemoving(null);
            remove.reset();
          }
        }}
        title={`Remove ${removing?.name ?? ''}?`}
        confirmLabel="Remove"
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => removing && remove.mutate(removing)}
      >
        <p>
          Arc forgets everything about this library: its {removing?.kind === 'shows' ? 'shows, episode orders' : kindUnit(removing?.kind)[1]} and anything
          you've edited. Your files aren't touched, and you can add the folders again at any time.
        </p>
        <p>A backup is made first, so you can undo this from Backups.</p>
      </ConfirmDialog>
    </Page>
  );
}

function LibraryRow({ lib, onEdit, onRemove }: { lib: Library; onEdit: () => void; onRemove: () => void }) {
  const Icon = kindIcon(lib.kind);
  const what = kindUnit(lib.kind);
  return (
    <div className="flex items-center gap-4 px-4 py-3.5">
      <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-surface-2 text-muted">
        <Icon className="size-5" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold">
          {lib.name} <span className="font-normal text-muted">· {fmt.plural(lib.count, what[0], what[1])}</span>
        </p>
        {lib.paths.map((p) => (
          <p key={p} className="truncate font-mono text-[11px] text-subtle" title={p}>
            {p}
          </p>
        ))}
        {lib.scan.running && (
          <p className="mt-0.5 flex items-center gap-1.5 text-xs text-accent">
            <Loader2 className="size-3 animate-spin" /> {fmt.scanProgress(lib.scan)}
          </p>
        )}
      </div>
      <Button variant="secondary" size="sm" onClick={onEdit}>
        <Pencil /> Edit
      </Button>
      <Button variant="ghost" size="icon" onClick={onRemove} aria-label={`Remove ${lib.name}`} title="Remove library">
        <Trash2 />
      </Button>
    </div>
  );
}
