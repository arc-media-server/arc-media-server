import type { Item, Library } from '@/lib/api';
import { cn } from '@/lib/utils';

// The providers' official logos, if they have been added to src/assets/attribution (see its
// README). Until then their names show as text, which still credits and links to them.
const logoFiles = import.meta.glob<string>('../assets/attribution/*.{svg,png}', { eager: true, query: '?url', import: 'default' });
const logo = (name: 'tmdb' | 'thetvdb') => Object.entries(logoFiles).find(([path]) => path.includes(`/${name}.`))?.[1];

type Provider = 'tmdb' | 'tvdb';

export const providers: Record<Provider, { name: string; logo?: string; home: string }> = {
  tmdb: { name: 'TMDB', logo: logo('tmdb'), home: 'https://www.themoviedb.org' },
  tvdb: { name: 'TheTVDB', logo: logo('thetvdb'), home: 'https://thetvdb.com' },
};

function ProviderMark({ provider, href, className }: { provider: Provider; href: string; className?: string }) {
  const p = providers[provider];
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className={cn('inline-flex items-center font-medium text-foreground/80 underline-offset-4 hover:underline', className)}>
      {p.logo ? <img src={p.logo} alt={p.name} className="h-3.5 w-auto" /> : p.name}
    </a>
  );
}

/**
 * The credit TMDB and TheTVDB ask for where their information is shown: which of them a movie's
 * or show's details (and a show's episode information) came from, linked to their site.
 */
export function ProviderCredit({ item, library }: { item: Item; library?: Library }) {
  if (item.metadata_state !== 'matched' && item.metadata_state !== 'locked') return null;
  if (item.kind === 'movie' && library?.kind === 'videos') return null;
  const lines: { label: string; provider: Provider; href: string }[] = [];
  const tmdbLink = item.tmdb_id ? `${providers.tmdb.home}/${item.kind === 'movie' ? 'movie' : 'tv'}/${item.tmdb_id}` : providers.tmdb.home;
  if (item.kind === 'movie') {
    if (item.tmdb_id) lines.push({ label: 'Details from', provider: 'tmdb', href: tmdbLink });
  } else {
    const details: Provider = library?.details_provider === 'tvdb' ? 'tvdb' : 'tmdb';
    const episodes: Provider = (item.source_ordering || library?.source_ordering || 'tvdb_aired').startsWith('tmdb') ? 'tmdb' : 'tvdb';
    const link = (p: Provider) => (p === 'tmdb' ? tmdbLink : providers.tvdb.home);
    if (details === episodes) lines.push({ label: 'Details and episodes from', provider: details, href: link(details) });
    else {
      lines.push({ label: 'Details from', provider: details, href: link(details) });
      lines.push({ label: 'Episode information from', provider: episodes, href: link(episodes) });
    }
  }
  if (!lines.length) return null;
  return (
    <p className="mx-6 mt-10 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-subtle md:mx-10">
      {lines.map((l) => (
        <span key={l.label} className="inline-flex items-center gap-1.5">
          {l.label} <ProviderMark provider={l.provider} href={l.href} />
        </span>
      ))}
    </p>
  );
}

/** Settings › About: the credits and notices TMDB and TheTVDB require. */
export function ProviderCredits() {
  return (
    <div className="space-y-4">
      <div className="flex items-start gap-4">
        <div className="flex w-24 shrink-0 justify-center pt-0.5"><ProviderMark provider="tmdb" href={providers.tmdb.home} className="text-sm" /></div>
        <p className="text-sm text-muted">This product uses the TMDB API but is not endorsed or certified by TMDB. Movie and show details, artwork and cast come from The Movie Database (TMDB).</p>
      </div>
      <div className="flex items-start gap-4">
        <div className="flex w-24 shrink-0 justify-center pt-0.5"><ProviderMark provider="tvdb" href={providers.tvdb.home} className="text-sm" /></div>
        <p className="text-sm text-muted">Metadata provided by <a href={providers.tvdb.home} target="_blank" rel="noopener noreferrer" className="font-medium text-foreground/80 underline-offset-4 hover:underline">TheTVDB</a>.</p>
      </div>
    </div>
  );
}
