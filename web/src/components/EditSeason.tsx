import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { api, type Item, type Season } from '@/lib/api';
import { cn } from '@/lib/utils';
import { applySeasonArtwork, SeasonPicker, type ArtworkChange } from '@/components/ChooseArtwork';
import { LockToggle } from '@/components/EditDetails';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent } from '@/components/ui/dialog';
import { inputClass } from '@/components/ui/field';

type Field = 'name' | 'overview';

/**
 * Edit a season as it appears in an episode order: its name and summary (locked once edited, so
 * refreshes keep them) and, in aired orders, its poster.
 */
export function EditSeasonDialog({
  series,
  season,
  orderingId,
  aired,
  open,
  onOpenChange,
}: {
  series: Item;
  season: Season;
  orderingId?: number;
  /** The order is an aired one, which season posters belong to. */
  aired: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && <SeasonForm series={series} season={season} orderingId={orderingId} aired={aired} onDone={() => onOpenChange(false)} />}
    </Dialog>
  );
}

function SeasonForm({ series, season, orderingId, aired, onDone }: { series: Item; season: Season; orderingId?: number; aired: boolean; onDone: () => void }) {
  const { t } = useTranslation('details');
  const qc = useQueryClient();
  // Season posters belong to the aired seasons, so they can only be changed in an aired order.
  const posters = aired;
  const [tab, setTab] = useState<'details' | 'poster'>('details');
  const [name, setName] = useState(season.name);
  const [overview, setOverview] = useState(season.overview ?? '');
  const [locked, setLocked] = useState(() => new Set<Field>(season.locked_fields));
  const [poster, setPoster] = useState<ArtworkChange>();

  const save = useMutation({
    mutationFn: async () => {
      const edit: { name?: string; overview?: string; locked_fields: string[] } = { locked_fields: [...locked] };
      if (name.trim() !== season.name) edit.name = name.trim();
      if (overview.trim() !== (season.overview ?? '')) edit.overview = overview.trim();
      await api.editSeason(series.id, season.season_number, orderingId, edit);
      if (poster) await applySeasonArtwork(series.id, season.season_number, poster);
    },
    onSuccess: () => {
      for (const key of ['seasons', 'artwork', 'items', 'continue', 'next-up']) qc.invalidateQueries({ queryKey: [key] });
      onDone();
    },
  });

  const edit = (field: Field, set: (v: string) => void) => (v: string) => {
    set(v);
    setLocked((prev) => new Set(prev).add(field));
  };
  const toggle = (field: Field) =>
    setLocked((prev) => {
      const next = new Set(prev);
      if (next.has(field)) next.delete(field);
      else next.add(field);
      return next;
    });

  return (
    <DialogContent
      title={t('details:editSeason.title', { name: season.name })}
      className={tab === 'poster' ? 'w-[min(820px,calc(100vw-32px))]' : undefined}
      description={tab === 'details' ? t("details:editSeason.anythingYouChangeIs") : undefined}
      footer={
        <>
          {save.error && <p className="mr-auto text-sm text-warning">{save.error.message}</p>}
          <DialogClose asChild>
            <Button variant="ghost" disabled={save.isPending}>{t("details:editSeason.cancel")}</Button>
          </DialogClose>
          <Button variant="primary" onClick={() => save.mutate()} disabled={save.isPending || !name.trim()}>
            {save.isPending && <Loader2 className="animate-spin" />} {t("details:editSeason.save")}</Button>
        </>
      }
    >
      {posters && (
        <div className="mb-5 flex gap-1" role="tablist">
          {(['details', 'poster'] as const).map((tabId) => (
            <button
              key={tabId}
              type="button"
              role="tab"
              aria-selected={tab === tabId}
              onClick={() => setTab(tabId)}
              className={cn('rounded-full px-3.5 py-1 text-[13px] font-medium transition-colors', tab === tabId ? 'bg-foreground text-background' : 'text-muted hover:text-foreground')}
            >
              {tabId === 'details' ? t("details:editSeason.details") : t("details:editSeason.poster")}
              {tabId === 'poster' && poster && <span className="ml-1.5 inline-block size-1.5 rounded-full bg-accent align-middle" aria-label={t("details:editSeason.changed")} />}
            </button>
          ))}
        </div>
      )}
      {tab === 'poster' ? (
        <SeasonPicker
          item={series}
          season={{ season_number: season.season_number, name: season.name, chosen: false, local: false }}
          change={poster}
          onChange={setPoster}
        />
      ) : (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate();
          }}
        >
          <label className="block">
            <span className="mb-1.5 flex items-center justify-between">
              <span className="text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("details:editSeason.name")}</span>
              <LockToggle locked={locked.has('name')} onToggle={() => toggle('name')} />
            </span>
            <input value={name} onChange={(e) => edit('name', setName)(e.target.value)} className={inputClass} />
          </label>
          <label className="block">
            <span className="mb-1.5 flex items-center justify-between">
              <span className="text-[11px] font-semibold uppercase tracking-wider text-subtle">{t("details:editSeason.summary")}</span>
              <LockToggle locked={locked.has('overview')} onToggle={() => toggle('overview')} />
            </span>
            <textarea
              value={overview}
              onChange={(e) => edit('overview', setOverview)(e.target.value)}
              rows={5}
              placeholder={t("details:editSeason.noSummaryYet")}
              className={cn(inputClass, 'h-auto resize-y py-2.5 leading-relaxed')}
            />
          </label>
          <button type="submit" hidden />
        </form>
      )}
    </DialogContent>
  );
}
