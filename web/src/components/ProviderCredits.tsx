import { Trans, useTranslation } from 'react-i18next';
import type { Item, Library } from '@/lib/api';
import { cn } from '@/lib/utils';
import tmdbLogo from '@/assets/attribution/tmdb.svg';
import tvdbLightLogo from '@/assets/attribution/thetvdb-light.png';
import tvdbDarkLogo from '@/assets/attribution/thetvdb-dark.png';

// Original provider assets are bundled, so credits also work without external image requests.
// Provenance and hashes: assets/attribution/README.md. Missing assets must fail the build.

type Provider = 'tmdb' | 'tvdb';

export const providers: Record<Provider, { name: string; logo: string; darkLogo?: string; home: string }> = {
  tmdb: { name: 'TMDB', logo: tmdbLogo, home: 'https://www.themoviedb.org' },
  tvdb: { name: 'TheTVDB', logo: tvdbLightLogo, darkLogo: tvdbDarkLogo, home: 'https://thetvdb.com' },
};

function ProviderMark({ provider, href, className, large = false }: { provider: Provider; href: string; className?: string; large?: boolean }) {
  const p = providers[provider];
  const imageClass = cn('object-contain', large ? 'h-6 w-20' : 'h-4 w-14');
  return (
    <a href={href} target="_blank" rel="noopener noreferrer" className={cn('inline-flex items-center font-medium text-foreground/80 underline-offset-4 hover:underline', className)}>
      <img src={p.logo} alt={p.name} className={cn(imageClass, p.darkLogo && 'provider-logo-light')} />
      {p.darkLogo && <img src={p.darkLogo} alt={p.name} className={cn(imageClass, 'provider-logo-dark')} />}
    </a>
  );
}

/**
 * The credit TMDB and TheTVDB ask for where their information is shown: which of them a movie's
 * or show's details (and a show's episode information) came from, linked to their site.
 */
export function ProviderCredit({ item, library }: { item: Item; library?: Library }) {
  useTranslation('profileSettings');
  if (item.metadata_state !== 'matched' && item.metadata_state !== 'locked') return null;
  if (item.kind === 'movie' && library?.kind === 'videos') return null;
  const lines: { label: string; provider: Provider; href: string }[] = [];
  const tmdbLink = providers.tmdb.home;
  if (item.kind === 'movie') {
    if (item.tmdb_id) lines.push({ label: 'details', provider: 'tmdb', href: tmdbLink });
  } else {
    const details: Provider = library?.details_provider === 'tvdb' ? 'tvdb' : 'tmdb';
    const episodes: Provider = (item.source_ordering || library?.source_ordering || 'tvdb_aired').startsWith('tmdb') ? 'tmdb' : 'tvdb';
    const link = (p: Provider) => (p === 'tmdb' ? tmdbLink : providers.tvdb.home);
    if (details === episodes) lines.push({ label: 'detailsEpisodes', provider: details, href: link(details) });
    else {
      lines.push({ label: 'details', provider: details, href: link(details) });
      lines.push({ label: 'episodes', provider: episodes, href: link(episodes) });
    }
  }
  if (!lines.length) return null;
  return (
    <p className="mx-6 mt-10 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-subtle md:mx-10">
      {lines.map((l) => (
        <span key={l.label} className="inline-flex items-center gap-1.5">
          {l.label === 'details' ? <Trans ns="profileSettings" i18nKey="providerCredits.details" components={{ provider: <ProviderMark provider={l.provider} href={l.href} /> }} /> : l.label === 'detailsEpisodes' ? <Trans ns="profileSettings" i18nKey="providerCredits.detailsEpisodes" components={{ provider: <ProviderMark provider={l.provider} href={l.href} /> }} /> : <Trans ns="profileSettings" i18nKey="providerCredits.episodes" components={{ provider: <ProviderMark provider={l.provider} href={l.href} /> }} />}
        </span>
      ))}
    </p>
  );
}

/** Settings › About: the credits and notices TMDB and TheTVDB require. */
export function ProviderCredits() {
  const { t } = useTranslation('profileSettings');
  return (
    <div className="space-y-4">
      <div className="flex items-start gap-4">
        <div className="flex w-24 shrink-0 justify-center pt-0.5"><ProviderMark provider="tmdb" href={providers.tmdb.home} large /></div>
        <p className="text-xs leading-relaxed text-muted">{t("profileSettings:providerCredits.thisProductUsesThe")}</p>
      </div>
      <div className="flex items-start gap-4">
        <div className="flex w-24 shrink-0 justify-center pt-0.5"><ProviderMark provider="tvdb" href={providers.tvdb.home} large /></div>
        <p className="text-xs leading-relaxed text-muted"><Trans ns="profileSettings" i18nKey="providerCredits.tvdbNotice" components={{ provider: <a href={providers.tvdb.home} target="_blank" rel="noopener noreferrer" className="font-medium text-foreground/80 underline-offset-4 hover:underline" /> }} /></p>
      </div>
    </div>
  );
}
