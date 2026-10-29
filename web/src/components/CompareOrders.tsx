import i18n from '@/i18n';
import { useTranslation } from 'react-i18next';
import { useMemo, useState } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { ArrowRight } from 'lucide-react';
import { api, type Ordering, type SeasonsResponse } from '@/lib/api';
import { cn } from '@/lib/utils';
import { ErrorNotice } from './Page';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

/** Compare two episode orders side by side before switching to one. */
export function CompareOrdersDialog({
  seriesId,
  orderings,
  current,
  open,
  onOpenChange,
  onUse,
  staged = false,
}: {
  seriesId: string;
  orderings: Ordering[];
  current: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUse: (orderingId: number) => void | Promise<unknown>;
  staged?: boolean;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <Comparison seriesId={seriesId} orderings={orderings} current={current} staged={staged} onClose={() => onOpenChange(false)} onUse={onUse} />}
    </Dialog>
  );
}

interface Place {
  season: number;
  episode: number;
}

interface Row {
  id: string;
  title: string;
  left?: Place;
  right?: Place;
}

function places(data?: SeasonsResponse) {
  const m = new Map<string, Place>();
  for (const s of data?.seasons ?? []) {
    for (const e of s.episodes) if (!m.has(e.id)) m.set(e.id, { season: s.season_number, episode: e.episode_number });
  }
  return m;
}

function code(p?: Place) {
  if (!p) return null;
  return p.season === 0 ? i18n.t('details:compareOrders.special', { number: p.episode }) : `S${p.season} E${p.episode}`;
}

function Comparison({
  seriesId,
  orderings,
  current,
  onClose,
  onUse,
  staged,
}: {
  seriesId: string;
  orderings: Ordering[];
  current: number;
  onClose: () => void;
  onUse: (orderingId: number) => void | Promise<unknown>;
  staged: boolean;
}) {
  const { t } = useTranslation('details');
  const [left, setLeft] = useState(current);
  const [right, setRight] = useState(() => orderings.find((o) => o.id !== current)?.id ?? current);
  const [changedOnly, setChangedOnly] = useState(false);
  const apply = useMutation({ mutationFn: () => Promise.resolve(onUse(right)), onSuccess: onClose });
  const a = useQuery({ queryKey: ['seasons', seriesId, 'compare', left], queryFn: () => api.seasons(seriesId, { ordering: left }) });
  const b = useQuery({ queryKey: ['seasons', seriesId, 'compare', right], queryFn: () => api.seasons(seriesId, { ordering: right }) });

  const rows = useMemo(() => {
    if (!a.data || !b.data) return null;
    const pa = places(a.data);
    const pb = places(b.data);
    const titles = new Map<string, string>();
    for (const d of [a.data, b.data]) {
      for (const s of d.seasons) for (const e of s.episodes) titles.set(e.id, e.title);
      for (const e of d.unplaced) titles.set(e.id, e.title);
    }
    const out: Row[] = [...titles.keys()].map((id) => ({ id, title: titles.get(id)!, left: pa.get(id), right: pb.get(id) }));
    // In the left order, then anything only the right order has.
    const key = (r: Row) => {
      const p = r.left ?? r.right;
      return p ? (r.left ? 0 : 1e9) + (p.season === 0 ? 1e6 : p.season * 1000) + p.episode : 2e9;
    };
    return out.sort((x, y) => key(x) - key(y));
  }, [a.data, b.data]);

  const differs = (r: Row) => r.left?.season !== r.right?.season || r.left?.episode !== r.right?.episode;
  const changed = rows?.filter(differs).length ?? 0;
  const shown = changedOnly ? rows?.filter(differs) : rows;
  const nameOf = (id: number) => orderings.find((o) => o.id === id)?.name ?? t('details:compareOrders.order');
  const error = a.error ?? b.error;

  return (
    <DialogContent
      title={t("details:compareOrders.compareEpisodeOrders")}
      description={staged ? t("details:compareOrders.stagedDescription") : t("details:compareOrders.seeWhereEachOf")}
      className="w-[min(820px,calc(100vw-32px))]"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{t("details:compareOrders.close")}</Button>
          <Button
            variant="primary"
            disabled={right === current || apply.isPending}
            onClick={() => apply.mutate()}
          >
            {right === current ? (staged ? t("details:compareOrders.alreadySelected") : t("details:compareOrders.alreadyInUse")) : t('details:compareOrders.useOrder', { name: nameOf(right) })}
          </Button>
        </>
      }
    >
      <div className="mb-3 grid grid-cols-[1fr_auto_1fr] items-center gap-3">
        <OrderSelect orderings={orderings} value={left} current={current} staged={staged} onChange={setLeft} />
        <ArrowRight className="size-4 text-subtle" />
        <OrderSelect orderings={orderings} value={right} current={current} staged={staged} onChange={setRight} />
      </div>
      <div className="mb-3 flex items-center justify-between gap-3 text-sm">
        <p className="text-muted">
          {rows
            ? changed === 0
              ? t('details:compareOrders.samePlace', { count: rows.length })
              : t('details:compareOrders.differentPlace', { count: changed, total: rows.length, episodes: t('details:episodeOrdering.episodes', { count: rows.length }) })
            : t("details:compareOrders.matchingEpisodes")}
        </p>
        <label className="flex shrink-0 cursor-pointer items-center gap-2 text-muted">
          <input type="checkbox" checked={changedOnly} onChange={(e) => setChangedOnly(e.target.checked)} className="accent-[var(--color-accent)]" />{t("details:compareOrders.onlyDifferences")}</label>
      </div>
      <ErrorNotice error={apply.error} onRetry={() => apply.mutate()} />
      {error && <p className="text-sm text-warning">{error.message}</p>}
      {!rows && !error && <div className="h-72 animate-pulse rounded-2xl bg-surface-2" />}
      {shown && (
        <div className="max-h-[52vh] overflow-auto rounded-2xl ring-1 ring-border">
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-surface-2 text-left text-[11px] font-semibold uppercase tracking-wider text-subtle">
              <tr>
                <th className="w-28 px-3 py-2">{nameOf(left)}</th>
                <th className="w-28 px-3 py-2">{nameOf(right)}</th>
                <th className="px-3 py-2">{t("details:compareOrders.episode")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {shown.map((r) => (
                <tr key={r.id} className={cn(differs(r) && 'bg-accent/[0.07]')}>
                  <td className="px-3 py-2 tabular-nums text-muted">{code(r.left) ?? <span className="text-subtle">{t("details:compareOrders.notIncluded")}</span>}</td>
                  <td className={cn('px-3 py-2 tabular-nums', differs(r) ? 'font-semibold text-accent' : 'text-muted')}>
                    {code(r.right) ?? <span className="font-normal text-subtle">{t("details:compareOrders.notIncluded")}</span>}
                  </td>
                  <td className="px-3 py-2">{r.title}</td>
                </tr>
              ))}
              {shown.length === 0 && (
                <tr>
                  <td colSpan={3} className="px-3 py-8 text-center text-muted">{t("details:compareOrders.noDifferences")}</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </DialogContent>
  );
}

function OrderSelect({ orderings, value, current, staged, onChange }: { orderings: Ordering[]; value: number; current: number; staged: boolean; onChange: (id: number) => void }) {
  const { t } = useTranslation('details');
  return (
    <select
      value={value}
      onChange={(e) => onChange(Number(e.target.value))}
      className={cn(inputClass, 'min-w-0')}
    >
      {orderings.map((o) => (
        <option key={o.id} value={o.id}>
          {o.name}
          {o.description ? ` (${o.description})` : ''}
          {o.id === current ? (staged ? t("details:compareOrders.selected") : t("details:compareOrders.inUse")) : ''}
        </option>
      ))}
    </select>
  );
}
