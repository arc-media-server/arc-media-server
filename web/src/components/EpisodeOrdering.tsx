import { useTranslation } from 'react-i18next';
import { useId } from 'react';
import { GitCompareArrows } from 'lucide-react';
import type { Ordering } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { FieldLabel, inputClass } from '@/components/ui/field';

/** Staged form fields; the enclosing Edit dialog owns Save/Cancel and error recovery. */
export function EpisodeOrderingFields({
  orderings, selected, pending, onSelect, onCompare, admin, specials, onSpecials,
}: {
  orderings: Ordering[];
  selected: number;
  pending: boolean;
  onSelect: (id: number | null) => void;
  onCompare?: () => void;
  admin: boolean;
  specials: 'inherit' | 'on' | 'off';
  onSpecials: (value: boolean | null) => void;
}) {
  const { t } = useTranslation('details');
  const id = useId();
  const groups: [string, Ordering[]][] = [
    ['TheTVDB', orderings.filter(o => o.source.startsWith('tvdb'))],
    ['TMDB', orderings.filter(o => o.source.startsWith('tmdb'))],
    [t('details:episodeOrdering.custom'), orderings.filter(o => o.source === 'custom')],
    [t('details:episodeOrdering.fileNames'), orderings.filter(o => o.source === 'file')],
  ];
  return <div className="space-y-6">
    <div>
      <label htmlFor={`${id}-display`}><FieldLabel>{t("details:episodeOrdering.displayOrder")}</FieldLabel></label>
      <select id={`${id}-display`} aria-describedby={`${id}-display-help`} value={selected || 'inherit'} disabled={pending}
        onChange={e => onSelect(e.target.value === 'inherit' ? null : Number(e.target.value))} className={inputClass}>
        <option value="inherit">{admin ? t("details:episodeOrdering.followFileOrder") : t("details:episodeOrdering.useServerDefault")}</option>
        {groups.filter(([, list]) => list.length).map(([label, list]) => <optgroup key={label} label={label}>
          {list.map(o => <option key={o.id} value={o.id}>
            {[providerLabel(o), o.name, t('details:episodeOrdering.episodes', { count: o.episode_count })].filter(Boolean).join(' · ')}
          </option>)}
        </optgroup>)}
      </select>
      <p id={`${id}-display-help`} className="mt-1.5 text-xs text-subtle">{t("details:episodeOrdering.changesBrowsingAndPlayback")}</p>
      {onCompare && <Button type="button" variant="ghost" size="sm" className="mt-2" disabled={pending} onClick={onCompare}>
        <GitCompareArrows className="size-4" /> {t("details:episodeOrdering.compareOrders")}</Button>}
    </div>
    <div>
      <label htmlFor={`${id}-specials`}><FieldLabel>{t("details:episodeOrdering.specialsInSeasons")}</FieldLabel></label>
      <select id={`${id}-specials`} aria-describedby={`${id}-specials-help`} value={specials} disabled={pending}
        onChange={e => onSpecials(e.target.value === 'inherit' ? null : e.target.value === 'on')} className={inputClass}>
        <option value="inherit">{admin ? t("details:episodeOrdering.defaultOff") : t("details:episodeOrdering.useServerSetting")}</option>
        <option value="on">{t("details:episodeOrdering.on")}</option>
        <option value="off">{t("details:episodeOrdering.off")}</option>
      </select>
      <p id={`${id}-specials-help`} className="mt-1.5 text-xs text-subtle">{t("details:episodeOrdering.placesSpecialsBetweenRegular")}</p>
    </div>
    <p className="text-xs text-subtle">{admin ? t("details:episodeOrdering.yourChoiceAlsoSets") : t("details:episodeOrdering.yourProfileOnly")}</p>
  </div>;
}

function providerLabel(o: Ordering) {
  if (o.source.startsWith('tvdb')) return 'TVDB';
  if (o.source.startsWith('tmdb')) return 'TMDB';
  return '';
}
