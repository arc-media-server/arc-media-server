import { useId } from 'react';
import { GitCompareArrows } from 'lucide-react';
import type { Ordering } from '@/lib/api';
import * as fmt from '@/lib/format';
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
  const id = useId();
  const groups: [string, Ordering[]][] = [
    ['TheTVDB', orderings.filter(o => o.source.startsWith('tvdb'))],
    ['TMDB', orderings.filter(o => o.source.startsWith('tmdb'))],
    ['Custom', orderings.filter(o => o.source === 'custom')],
    ['From file names', orderings.filter(o => o.source === 'file')],
  ];
  return <div className="space-y-6">
    <div>
      <label htmlFor={`${id}-display`}><FieldLabel>Display order</FieldLabel></label>
      <select id={`${id}-display`} aria-describedby={`${id}-display-help`} value={selected || 'inherit'} disabled={pending}
        onChange={e => onSelect(e.target.value === 'inherit' ? null : Number(e.target.value))} className={inputClass}>
        <option value="inherit">{admin ? 'Follow file order' : 'Use server default'}</option>
        {groups.filter(([, list]) => list.length).map(([label, list]) => <optgroup key={label} label={label}>
          {list.map(o => <option key={o.id} value={o.id}>
            {[providerLabel(o), o.name, fmt.plural(o.episode_count, 'episode')].filter(Boolean).join(' · ')}
          </option>)}
        </optgroup>)}
      </select>
      <p id={`${id}-display-help`} className="mt-1.5 text-xs text-subtle">Changes browsing and playback sequence. Your files keep the same episode identities.</p>
      {onCompare && <Button type="button" variant="ghost" size="sm" className="mt-2" disabled={pending} onClick={onCompare}>
        <GitCompareArrows className="size-4" /> Compare orders…
      </Button>}
    </div>
    <div>
      <label htmlFor={`${id}-specials`}><FieldLabel>Specials in seasons</FieldLabel></label>
      <select id={`${id}-specials`} aria-describedby={`${id}-specials-help`} value={specials} disabled={pending}
        onChange={e => onSpecials(e.target.value === 'inherit' ? null : e.target.value === 'on')} className={inputClass}>
        <option value="inherit">{admin ? 'Default (Off)' : 'Use server setting'}</option>
        <option value="on">On</option>
        <option value="off">Off</option>
      </select>
      <p id={`${id}-specials-help`} className="mt-1.5 text-xs text-subtle">Places specials between regular episodes when their position is known. Others stay in Specials; custom placements are kept.</p>
    </div>
    <p className="text-xs text-subtle">{admin ? 'Your choice also sets the server default' : 'Your profile only'}</p>
  </div>;
}

function providerLabel(o: Ordering) {
  if (o.source.startsWith('tvdb')) return 'TVDB';
  if (o.source.startsWith('tmdb')) return 'TMDB';
  return '';
}
