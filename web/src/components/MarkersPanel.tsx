import { useTranslation } from 'react-i18next';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2, RotateCcw } from 'lucide-react';
import { api, type Item, type Marker } from '@/lib/api';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { inputClass } from '@/components/ui/field';

/** "1:23" or "1:02:03" for milliseconds; "" for none. */
function clock(ms: number) {
  const s = Math.round(ms / 1000);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`;
}

/** Milliseconds from "1:23", "1:02:03" or "83"; null if it isn't a time. */
function parseClock(s: string): number | null {
  const t = s.trim();
  if (!/^\d+(:\d{1,2}){0,2}$/.test(t)) return null;
  return t.split(':').reduce((acc, p) => acc * 60 + Number(p), 0) * 1000;
}

export type MarkerEdit = { kind: string; start_ms: number; end_ms: number }[];

/**
 * Where an episode's or movie's recap, intro and credits are, for Skip Intro and Skip Credits.
 * Times typed here are kept: found ones never replace them.
 */
export function MarkersPanel({ item, onChange }: { item: Item; onChange: (edit: MarkerEdit | null, valid: boolean) => void }) {
  const { t } = useTranslation('metadata');
  const kinds = [['recap', t('metadata:markersPanel.recap')], ['intro', t('metadata:markersPanel.intro')], ['credits', t('metadata:markersPanel.credits')]] as const;
  const sourceNames: Record<Marker['source'], string> = {
    manual: t('metadata:markersPanel.manual'),
    chapters: t('metadata:markersPanel.chapters'),
    theintrodb: t('metadata:markersPanel.theintrodb'),
    introdb: t('metadata:markersPanel.introdb'),
    detected: t('metadata:markersPanel.detected'),
  };
  const qc = useQueryClient();
  const { data, error } = useQuery({ queryKey: ['markers', item.id], queryFn: () => api.itemMarkers(item.id) });
  const [times, setTimes] = useState<Record<string, { start: string; end: string }> | null>(null);
  useEffect(() => {
    if (data && !times) {
      const t: Record<string, { start: string; end: string }> = {};
      for (const [k] of kinds) {
        const m = data.markers.find((x) => x.kind === k && x.end_ms > x.start_ms);
        t[k] = m ? { start: clock(m.start_ms), end: clock(m.end_ms) } : { start: '', end: '' };
      }
      setTimes(t);
    }
  }, [data, times]);
  const reset = useMutation({
    mutationFn: () => api.resetItemMarkers(item.id),
    onSuccess: () => {
      setTimes(null);
      onChange(null, true);
      qc.invalidateQueries({ queryKey: ['markers', item.id] });
    },
  });

  if (error) return <p className="text-sm text-warning">{error.message}</p>;
  if (!data || !times) return <div className="h-48 animate-pulse rounded-2xl bg-surface-2" />;

  const update = (kind: string, field: 'start' | 'end', value: string) => {
    const next = { ...times, [kind]: { ...times[kind], [field]: value } };
    setTimes(next);
    const edit: MarkerEdit = [];
    let valid = true;
    for (const [k] of kinds) {
      const { start, end } = next[k];
      if (!start.trim() && !end.trim()) continue;
      const s = parseClock(start);
      const e = parseClock(end);
      if (s === null || e === null || e <= s || (data.duration_ms > 0 && e > data.duration_ms + 1000)) valid = false;
      else edit.push({ kind: k, start_ms: s, end_ms: e });
    }
    onChange(edit, valid);
  };

  return (
    <div className="space-y-4 pb-2">
      <p className="text-sm text-muted">{data.duration_ms ? t('metadata:markersPanel.descriptionDuration', { duration: clock(data.duration_ms) }) : t('metadata:markersPanel.description')}</p>
      <div className="divide-y divide-border overflow-hidden rounded-2xl bg-surface-2/50 ring-1 ring-border">
        {kinds.map(([k, label]) => {
          const found = data.markers.find((m) => m.kind === k);
          const { start, end } = times[k];
          const bad = (start.trim() || end.trim()) && (parseClock(start) === null || parseClock(end) === null || (parseClock(end) ?? 0) <= (parseClock(start) ?? 0));
          return (
            <div key={k} className="flex flex-wrap items-center gap-3 px-4 py-3">
              <div className="w-36 min-w-0">
                <p className="text-sm font-medium">{label}</p>
                <p className="truncate text-xs text-subtle">
                  {found ? (found.end_ms > found.start_ms ? sourceNames[found.source] : t("metadata:markersPanel.noneSetByYou")) : item.kind === 'episode' ? t("metadata:markersPanel.notFoundYet") : t("metadata:markersPanel.notSet")}
                </p>
              </div>
              <input value={start} onChange={(e) => update(k, 'start', e.target.value)} placeholder={t("metadata:markersPanel.start105")} className={cn(inputClass, 'h-9 w-32 font-mono text-sm', bad && 'ring-warning')} />
              <span className="text-subtle">–</span>
              <input value={end} onChange={(e) => update(k, 'end', e.target.value)} placeholder={t("metadata:markersPanel.end235")} className={cn(inputClass, 'h-9 w-32 font-mono text-sm', bad && 'ring-warning')} />
            </div>
          );
        })}
      </div>
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" disabled={reset.isPending} onClick={() => reset.mutate()}>
          {reset.isPending ? <Loader2 className="animate-spin" /> : <RotateCcw />} {t("metadata:markersPanel.findAgain")}</Button>
        <p className="text-xs text-subtle">{t("metadata:markersPanel.forgetsTheseTimesAnd")}</p>
      </div>
    </div>
  );
}
