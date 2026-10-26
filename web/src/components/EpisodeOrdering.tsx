import { ChevronDown, GitCompareArrows, ListOrdered, Loader2 } from 'lucide-react';
import type { Ordering } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
export function OrderingMenu({
  orderings,
  selected,
  pending,
  onSelect,
  onCompare,
  admin, onReset,
  inlineSpecials, onSpecials, specialsPending,
}: {
  orderings: Ordering[];
  selected: number;
  pending: boolean;
  onSelect: (id: number) => void;
  onCompare?: () => void;
  admin: boolean;
  onReset: () => void;
  inlineSpecials: boolean;
  onSpecials: (inline: boolean | null) => void;
  specialsPending: boolean;
}) {
  const current = orderings.find((o) => o.id === selected);
  const groups: [string, Ordering[]][] = [
    ['TheTVDB', orderings.filter((o) => o.source.startsWith('tvdb'))],
    ['TMDB', orderings.filter((o) => o.source.startsWith('tmdb'))],
    ['Custom', orderings.filter((o) => o.source === 'custom')],
    ['From File Names', orderings.filter((o) => o.source === 'file')],
  ];
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="secondary" size="sm" disabled={pending}>
          {pending ? <Loader2 className="animate-spin" /> : <ListOrdered />}
          {pending ? 'Matching episodes…' : (current?.name ?? (admin ? 'Follow file order' : 'Use server default'))}
          {current && !pending && <span className="text-subtle">{providerLabel(current)}</span>}
          <ChevronDown className="text-subtle" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <DropdownMenuRadioGroup value={String(selected)} onValueChange={(v) => onSelect(Number(v))}>
          {groups
            .filter(([, list]) => list.length)
            .map(([label, list], i) => (
              <div key={label}>
                {i > 0 && <DropdownMenuSeparator />}
                <DropdownMenuLabel>{label}</DropdownMenuLabel>
                {list.map((o) => (
                  <DropdownMenuRadioItem key={o.id} value={String(o.id)} aria-label={`${o.name}${providerLabel(o) ? ` (${providerLabel(o)})` : ''}`}>
                    <div className="min-w-0">
                      <p className="truncate">{o.name}</p>
                      <p className="text-xs text-subtle">
                        {[o.description?.replace(/^(TVDB|TMDB)( · )?/, ''), fmt.plural(o.episode_count, 'episode')].filter(Boolean).join(' · ')}
                      </p>
                    </div>
                  </DropdownMenuRadioItem>
                ))}
              </div>
            ))}
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuCheckboxItem checked={inlineSpecials} disabled={specialsPending} onCheckedChange={(checked) => onSpecials(checked)}>
          Show specials in seasons
        </DropdownMenuCheckboxItem>
        <DropdownMenuItem disabled={specialsPending} onSelect={() => onSpecials(null)}>{admin ? 'Reset specials to Off' : 'Use server specials setting'}</DropdownMenuItem>
        <DropdownMenuLabel className="whitespace-normal text-xs font-normal">Uses TheTVDB placement hints from metadata refresh. Unknown specials stay in Specials. Manual/custom orders stay as arranged.</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuLabel>{admin ? 'Your choice also sets the server default' : 'Your profile only'}</DropdownMenuLabel>
        <DropdownMenuItem onSelect={onReset}>{admin ? 'Follow file order' : 'Use server default'}</DropdownMenuItem>
        {onCompare && (
          <DropdownMenuItem onSelect={onCompare}>
            <GitCompareArrows className="size-4 text-muted" /> Compare Orders…
          </DropdownMenuItem>
        )}
        <DropdownMenuItem disabled className="text-xs text-subtle">
          Changes how episodes are shown. Your files aren't touched.
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function providerLabel(o: Ordering) {
  if (o.source.startsWith('tvdb')) return 'TVDB';
  if (o.source.startsWith('tmdb')) return 'TMDB';
  return '';
}
