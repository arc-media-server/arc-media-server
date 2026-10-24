import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Loader2, Pencil, Plus, Send, Trash2, XCircle } from 'lucide-react';
import { api, type Webhook } from '@/lib/api';
import { cn } from '@/lib/utils';
import { ErrorNotice } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

const eventLabels: Record<string, string> = {
  'library.new': 'New movies and episodes',
  'playback.start': 'Someone starts watching',
  'playback.watched': 'Someone finishes watching',
  'server.problem': 'Problems, like a failed scan or backup',
};
const formatLabels = { discord: 'Discord', slack: 'Slack', json: 'JSON' };

/** Webhooks: where Arc posts events, for Discord, Slack, home automation or anything else. */
export function Webhooks() {
  const qc = useQueryClient();
  const webhooks = useQuery({ queryKey: ['webhooks'], queryFn: api.webhooks });
  const { data } = webhooks;
  const [editing, setEditing] = useState<Webhook | 'new' | null>(null);
  const [removing, setRemoving] = useState<Webhook | null>(null);
  const remove = useMutation({
    mutationFn: (h: Webhook) => api.deleteWebhook(h.id),
    onSuccess: () => (qc.invalidateQueries({ queryKey: ['webhooks'] }), setRemoving(null)),
  });
  const test = useMutation({ mutationFn: (h: Webhook) => api.testWebhook(h.id), onSettled: () => qc.invalidateQueries({ queryKey: ['webhooks'] }) });
  return (
    <>
      {webhooks.error && <div className="m-4 space-y-2">
        {data && <p className="text-xs text-muted">Showing the last loaded notifications. They may be out of date.</p>}
        <ErrorNotice error={webhooks.error} onRetry={() => void webhooks.refetch()} busy={webhooks.isFetching} />
      </div>}
      {!data && !webhooks.error && <p role="status" className="px-4 py-3 text-sm text-muted">Loading notifications…</p>}
      {data && data.webhooks.length === 0 && <p className="px-4 py-3 text-sm text-muted">No notifications configured yet.</p>}
      {data?.webhooks.map((h) => (
        <div key={h.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3.5">
          <div className="min-w-0 flex-1 basis-64">
            <p className="flex items-center gap-2 text-sm font-medium">
              {h.name}
              <span className="rounded-md bg-surface-3 px-1.5 py-px text-[10px] font-semibold uppercase tracking-wide text-muted">{formatLabels[h.format]}</span>
            </p>
            <p className="mt-0.5 truncate font-mono text-[11px] text-subtle" title={h.url}>
              {h.url}
            </p>
            <p className="mt-0.5 text-xs text-muted">
              {h.events.length === 0 || h.events.length === data.events.length ? 'Every event' : h.events.map((e) => eventLabels[e] ?? e).join(' · ')}
            </p>
            {h.last && (
              <p className={cn('mt-1 flex items-center gap-1 text-xs', h.last.ok ? 'text-subtle' : 'text-warning')}>
                {h.last.ok ? <CheckCircle2 className="size-3.5 text-accent" /> : <XCircle className="size-3.5" />}
                Last sent {new Date(h.last.at).toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' })}: {h.last.status}
              </p>
            )}
          </div>
          <div className="flex gap-1">
            <Button variant="ghost" size="sm" disabled={test.isPending} onClick={() => test.mutate(h)}>
              {test.isPending && test.variables?.id === h.id ? <Loader2 className="animate-spin" /> : <Send />} Test
            </Button>
            <Button variant="ghost" size="sm" aria-label="Edit" onClick={() => setEditing(h)}>
              <Pencil />
            </Button>
            <Button variant="ghost" size="sm" aria-label="Remove" onClick={() => setRemoving(h)}>
              <Trash2 />
            </Button>
          </div>
        </div>
      ))}
      <div className="flex items-center gap-4 px-4 py-3.5">
        <p className="min-w-0 flex-1 text-xs leading-relaxed text-muted">
          {!data ? 'Load notifications before adding a webhook.' : data.webhooks.length ? 'Add another' : 'Add a Discord or Slack webhook to get messages there, or any address to receive events as JSON'}
          {!data ? '' : data.webhooks.length ? '.' : ' (for home automation, ntfy and the like).'}
        </p>
        <Button variant="secondary" size="sm" disabled={!data} onClick={() => setEditing('new')}>
          <Plus /> Add Webhook
        </Button>
      </div>
      <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
        {editing && data && <WebhookForm webhook={editing === 'new' ? undefined : editing} events={data.events} onDone={() => setEditing(null)} />}
      </Dialog>
      <ConfirmDialog
        open={!!removing}
        onOpenChange={(o) => !o && setRemoving(null)}
        title="Remove Webhook"
        confirmLabel="Remove"
        destructive
        busy={remove.isPending}
        error={remove.error?.message}
        onConfirm={() => removing && remove.mutate(removing)}
      >
        Arc stops sending events to {removing?.name}.
      </ConfirmDialog>
    </>
  );
}

function WebhookForm({ webhook, events, onDone }: { webhook?: Webhook; events: string[]; onDone: () => void }) {
  const qc = useQueryClient();
  const [name, setName] = useState(webhook?.name ?? '');
  const [url, setUrl] = useState(webhook?.url ?? '');
  const [chosen, setChosen] = useState<string[]>(webhook?.events.length ? webhook.events : events);
  const save = useMutation({
    mutationFn: () => api.saveWebhook({ id: webhook?.id, name, url, events: chosen.length === events.length ? [] : chosen }),
    onSuccess: () => (qc.invalidateQueries({ queryKey: ['webhooks'] }), onDone()),
  });
  return (
    <DialogContent
      title={webhook ? 'Edit Webhook' : 'Add a Webhook'}
      description="Arc posts a message to this address when the events you choose happen."
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost">Cancel</Button>
          </DialogClose>
          <Button variant="primary" disabled={!url.trim() || chosen.length === 0 || save.isPending} onClick={() => save.mutate()}>
            {save.isPending && <Loader2 className="animate-spin" />} Save
          </Button>
        </>
      }
    >
      <div className="space-y-4 pb-2">
        <label className="block">
          <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Address</span>
          <input autoFocus value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://discord.com/api/webhooks/…" className={inputClass} />
          <span className="mt-1.5 block text-xs text-subtle">Discord and Slack addresses get a tidy message; anything else gets the event as JSON.</span>
        </label>
        <label className="block">
          <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Name</span>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Family Discord" maxLength={60} className={inputClass} />
        </label>
        <div>
          <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">Send</span>
          <div className="divide-y divide-border overflow-hidden rounded-xl bg-surface-2/50 ring-1 ring-border">
            {events.map((e) => (
              <label key={e} className="flex cursor-pointer items-center gap-3 px-3 py-2.5 text-sm">
                <input
                  type="checkbox"
                  checked={chosen.includes(e)}
                  onChange={(ev) => setChosen((c) => (ev.target.checked ? [...c, e] : c.filter((x) => x !== e)))}
                  className="size-4 accent-[var(--color-accent)]"
                />
                {eventLabels[e] ?? e}
              </label>
            ))}
          </div>
        </div>
      </div>
    </DialogContent>
  );
}
