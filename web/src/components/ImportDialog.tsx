import { useTranslation } from 'react-i18next';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ChevronDown, ExternalLink, Loader2, Server } from 'lucide-react';
import { api, type ImportSession } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

/** Bring watch history over from Plex or Jellyfin. */
export function ImportDialog({ source, open, onOpenChange }: { source: 'plex' | 'jellyfin'; open: boolean; onOpenChange: (o: boolean) => void }) {
  return <Dialog open={open} onOpenChange={onOpenChange}>{open && <ImportFlow source={source} onClose={() => onOpenChange(false)} />}</Dialog>;
}

const sourceName = { plex: 'Plex', jellyfin: 'Jellyfin' };

function ImportFlow({ source, onClose }: { source: 'plex' | 'jellyfin'; onClose: () => void }) {
  const { t } = useTranslation('accessSettings');
  const qc = useQueryClient();
  const [id, setId] = useState<string | null>(null);
  const session = useQuery({
    queryKey: ['import', id],
    queryFn: () => api.importSession(id!),
    enabled: !!id,
    // Waiting for someone to approve Arc on plex.tv, or for the import to finish.
    refetchInterval: (q) => (q.state.data?.stage === 'signing_in' || q.state.data?.stage === 'running' ? 2000 : false),
  });
  const s = session.data;
  useEffect(() => {
    if (s?.stage === 'done') {
      for (const key of ['continue', 'next-up', 'items', 'item', 'profiles', 'watchlist']) qc.invalidateQueries({ queryKey: [key] });
    }
  }, [s?.stage, qc]);

  const title = t('accessSettings:importDialog.fromSource', { source: sourceName[source] });
  const description = source === 'plex' ? t('accessSettings:importDialog.plexDescription') : t('accessSettings:importDialog.jellyfinDescription');

  if (!s) {
    return source === 'plex' ? (
      <PlexStart title={title} description={description} onStarted={(sess) => (qc.setQueryData(['import', sess.id], sess), setId(sess.id))} />
    ) : (
      <JellyfinStart title={title} description={description} onStarted={(sess) => (qc.setQueryData(['import', sess.id], sess), setId(sess.id))} />
    );
  }
  switch (s.stage) {
    case 'signing_in':
      return (
        <DialogContent title={title} description={description} footer={<CloseButton />}>
          <div className="flex items-center gap-3 rounded-2xl bg-surface-2/60 p-4 ring-1 ring-border">
            <Loader2 className="size-5 shrink-0 animate-spin text-accent" />
            <div className="min-w-0 flex-1 text-sm">
              <p className="font-medium">{t("accessSettings:importDialog.waitingForYouTo")}</p>
              <p className="text-xs text-muted">{t("accessSettings:importDialog.approveArcMediaServer")}</p>
            </div>
            <Button variant="secondary" size="sm" onClick={() => window.open(s.auth_url, '_blank')}>{t("accessSettings:importDialog.openPlex")} <ExternalLink />
            </Button>
          </div>
        </DialogContent>
      );
    case 'choose_server':
      return <ChooseServer title={title} session={s} />;
    case 'choose_accounts':
      return <ChooseAccounts title={title} session={s} />;
    case 'running':
      return (
        <DialogContent title={title} description={t("accessSettings:importDialog.thisRunsInThe")} footer={<CloseButton label={t("accessSettings:importDialog.close")} />}>
          <ImportProgress />
        </DialogContent>
      );
    default:
      return <Results title={title} session={s} onClose={onClose} />;
  }
}

function CloseButton({ label }: { label?: string }) {
  const { t } = useTranslation('accessSettings');
  return (
    <DialogClose asChild>
      <Button variant="ghost">{label ?? t('accessSettings:importDialog.cancel')}</Button>
    </DialogClose>
  );
}

function PlexStart({ title, description, onStarted }: { title: string; description: string; onStarted: (s: ImportSession) => void }) {
  const { t } = useTranslation('accessSettings');
  const start = useMutation({
    mutationFn: async () => {
      // Open the tab straight away so the browser doesn't block it, then point it at Plex.
      const tab = window.open('about:blank', '_blank');
      try {
        const s = await api.startImport({ source: 'plex' });
        if (tab && s.auth_url) tab.location.href = s.auth_url;
        return s;
      } catch (e) {
        tab?.close();
        throw e;
      }
    },
    onSuccess: onStarted,
  });
  return (
    <DialogContent
      title={title}
      description={description}
      footer={
        <>
          {start.error && <p className="mr-auto text-sm text-warning">{start.error.message}</p>}
          <CloseButton />
          <Button variant="primary" disabled={start.isPending} onClick={() => start.mutate()}>
            {start.isPending && <Loader2 className="animate-spin" />} {t("accessSettings:importDialog.signInWithPlex")}</Button>
        </>
      }
    >
      <p className="pb-2 text-sm text-muted">{t("accessSettings:importDialog.plexOpensInA")}</p>
    </DialogContent>
  );
}

function JellyfinStart({ title, description, onStarted }: { title: string; description: string; onStarted: (s: ImportSession) => void }) {
  const { t } = useTranslation('accessSettings');
  const [url, setUrl] = useState('http://localhost:8096');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const start = useMutation({ mutationFn: () => api.startImport({ source: 'jellyfin', url, username, password }), onSuccess: onStarted });
  return (
    <DialogContent
      title={title}
      description={description}
      footer={
        <>
          {start.error && <p className="mr-auto text-sm text-warning">{start.error.message}</p>}
          <CloseButton />
          <Button variant="primary" disabled={!url.trim() || !username || start.isPending} onClick={() => start.mutate()}>
            {start.isPending && <Loader2 className="animate-spin" />} {t("accessSettings:importDialog.connect")}</Button>
        </>
      }
    >
      <form
        className="space-y-4 pb-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (url.trim() && username) start.mutate();
        }}
      >
        <label className="block">
          <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("accessSettings:importDialog.serverAddress")}</span>
          <input value={url} onChange={(e) => setUrl(e.target.value)} className={inputClass} placeholder={t("accessSettings:importDialog.http1921681")} />
        </label>
        <div className="grid grid-cols-2 gap-3">
          <label className="block">
            <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("accessSettings:importDialog.administrator")}</span>
            <input value={username} onChange={(e) => setUsername(e.target.value)} className={inputClass} autoComplete="off" />
          </label>
          <label className="block">
            <span className="mb-1.5 block text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("accessSettings:importDialog.password")}</span>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} className={inputClass} autoComplete="off" />
          </label>
        </div>
        <p className="text-xs text-subtle">{t("accessSettings:importDialog.anAdministratorCanRead")}</p>
        <button type="submit" hidden />
      </form>
    </DialogContent>
  );
}

function ChooseServer({ title, session: s }: { title: string; session: ImportSession }) {
  const { t } = useTranslation('accessSettings');
  const qc = useQueryClient();
  const choose = useMutation({ mutationFn: (serverId: string) => api.chooseImportServer(s.id, serverId), onSuccess: (n) => qc.setQueryData(['import', s.id], n) });
  return (
    <DialogContent
      title={title}
      description={t("accessSettings:importDialog.whichPlexServerShould")}
      footer={
        <>
          {choose.error && <p className="mr-auto text-sm text-warning">{choose.error.message}</p>}
          <CloseButton />
        </>
      }
    >
      <div className="divide-y divide-border overflow-hidden rounded-2xl ring-1 ring-border">
        {s.servers?.map((sv) => (
          <button
            key={sv.id}
            type="button"
            disabled={choose.isPending}
            onClick={() => choose.mutate(sv.id)}
            className="flex w-full items-center gap-3 px-4 py-3 text-left text-sm transition-colors hover:bg-surface-2 disabled:opacity-60"
          >
            <Server className="size-4 text-muted" />
            <span className="flex-1 truncate font-medium">{sv.name}</span>
            <span className="text-xs text-subtle">{sv.owned ? t("accessSettings:importDialog.yours") : t("accessSettings:importDialog.sharedWithYou")}</span>
            {choose.isPending && choose.variables === sv.id && <Loader2 className="size-4 animate-spin" />}
          </button>
        ))}
      </div>
    </DialogContent>
  );
}

function ChooseAccounts({ title, session: s }: { title: string; session: ImportSession }) {
  const { t } = useTranslation('accessSettings');
  const qc = useQueryClient();
  const profiles = useQuery({ queryKey: ['profiles'], queryFn: api.profiles });
  const [choice, setChoice] = useState<Record<string, string>>(() =>
    Object.fromEntries((s.accounts ?? []).map((a) => [a.id, a.suggested_profile ?? ''])),
  );
  const run = useMutation({ mutationFn: () => api.runImport(s.id, choice), onSuccess: (n) => qc.setQueryData(['import', s.id], n) });
  const chosen = Object.values(choice).filter(Boolean).length;
  return (
    <DialogContent
      title={title}
      description={s.server ? t('accessSettings:importDialog.accountsFromServer', { server: s.server }) : t('accessSettings:importDialog.accountsDescription')}
      footer={
        <>
          {run.error && <p className="mr-auto text-sm text-warning">{run.error.message}</p>}
          <CloseButton />
          <Button variant="primary" disabled={!chosen || run.isPending} onClick={() => run.mutate()}>
            {run.isPending && <Loader2 className="animate-spin" />} {chosen > 0 ? t('accessSettings:importDialog.importCount', { count: chosen }) : t('accessSettings:importDialog.import')}
          </Button>
        </>
      }
    >
      <div className="space-y-2 pb-2">
        {s.accounts?.map((a) => (
          <div key={a.id} className="flex items-center gap-3 rounded-xl bg-surface-2/50 px-3 py-2.5 ring-1 ring-border">
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{a.name}</p>
              {s.source === 'plex' && (
                <p className="text-xs text-subtle">{a.owner ? t("accessSettings:importDialog.watchedResumePointsRatings") : t("accessSettings:importDialog.watchedAndHistory")}</p>
              )}
            </div>
            <div className="relative w-52 shrink-0">
              <select
                value={choice[a.id] ?? ''}
                onChange={(e) => setChoice((c) => ({ ...c, [a.id]: e.target.value }))}
                className={cn(inputClass, 'h-9 appearance-none pr-8 text-sm')}
              >
                <option value="">{t("accessSettings:importDialog.donTImport")}</option>
                <option value="new">{t('accessSettings:importDialog.namedProfile', { name: a.name })}</option>
                {profiles.data?.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
              <ChevronDown className="pointer-events-none absolute right-2.5 top-2.5 size-4 text-subtle" />
            </div>
          </div>
        ))}
        {s.source === 'plex' && (
          <p className="pt-1 text-xs text-subtle">{t("accessSettings:importDialog.plexOnlyLetsArc")}</p>
        )}
      </div>
    </DialogContent>
  );
}

function ImportProgress() {
  const { t } = useTranslation('accessSettings');
  const { data } = useQuery({ queryKey: ['activity'], queryFn: api.activity, refetchInterval: 1500 });
  const task = data?.running.find((tabId) => tabId.kind === 'import');
  const pct = task?.total ? Math.round(((task.done ?? 0) / task.total) * 100) : null;
  return (
    <div className="pb-2">
      <p className="flex items-center gap-2 text-sm font-medium">
        <Loader2 className="size-4 animate-spin text-accent" /> {task?.detail ?? t("accessSettings:importDialog.starting")}
      </p>
      <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-surface-3">
        <div className={cn('h-full rounded-full bg-accent transition-all', pct === null && 'w-1/3 animate-pulse')} style={pct === null ? undefined : { width: `${pct}%` }} />
      </div>
    </div>
  );
}

function Results({ title, session: s, onClose }: { title: string; session: ImportSession; onClose: () => void }) {
  const { t } = useTranslation('accessSettings');
  const [open, setOpen] = useState<string | null>(null);
  return (
    <DialogContent
      title={title}
      description={s.stage === 'failed' ? undefined : t("accessSettings:importDialog.doneContinueWatchingWatched")}
      footer={
        <Button variant="primary" onClick={onClose}>{t("accessSettings:importDialog.done")}</Button>
      }
    >
      <div className="space-y-2 pb-2">
        {s.error && <p className="rounded-xl bg-warning/10 px-3 py-2 text-sm text-warning">{s.error}</p>}
        {s.results?.map((r) => (
          <div key={r.account} className="rounded-xl bg-surface-2/50 px-3 py-2.5 ring-1 ring-border">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="size-4 shrink-0 text-accent" />
              <p className="min-w-0 flex-1 truncate text-sm font-medium">
                {r.account} <span className="font-normal text-muted">→ {r.profile}</span>
              </p>
            </div>
            <p className="mt-1 pl-6 text-xs text-muted">
              {[t('accessSettings:importDialog.matchedTitles', { count: r.matched }), r.history > 0 && t('accessSettings:importDialog.history', { count: r.history })].filter(Boolean).join(' · ')}
              {r.not_found > 0 && (
                <>
                  {' · '}
                  <button type="button" className="text-foreground/80 underline-offset-2 hover:underline" onClick={() => setOpen(open === r.account ? null : r.account)}>
                    {t('accessSettings:importDialog.missingCount', { count: r.not_found })}</button>
                </>
              )}
            </p>
            {open === r.account && r.missing && (
              <ul className="mt-2 max-h-40 space-y-0.5 overflow-y-auto pl-6 text-xs text-subtle">
                {r.missing.map((m, i) => (
                  <li key={i} className="truncate">
                    {m}
                  </li>
                ))}
                {r.not_found > r.missing.length && <li>{t('accessSettings:importDialog.moreMissing', { count: r.not_found - r.missing.length })}</li>}
              </ul>
            )}
          </div>
        ))}
      </div>
    </DialogContent>
  );
}
