import { useState } from 'react';
import type { Item } from '@/lib/api';
import { Artwork } from './Artwork';

/**
 * The artwork of a backdrop card (the "Backdrops" style): a squarer card with the title's background art (the show's, for an
 * episode) and its logo, as on Apple TV. Background art never gives an episode away, so spoiler
 * protection has nothing to hide here. Without a logo, the title is shown on the art instead.
 */
export function ArcArtwork({ item, title, image }: { item: Item; title: string; image?: string }) {
  const isEpisode = item.kind === 'episode';
  const art = image ?? (isEpisode ? (item.images?.series_backdrop ?? item.images?.primary) : (item.images?.backdrop ?? item.images?.primary));
  const logo = isEpisode ? item.images?.series_logo : item.images?.logo;
  const [logoFailed, setLogoFailed] = useState(false);
  const showLogo = logo && !logoFailed;
  return (
    <>
      <Artwork src={art} alt="" kind="still" icon={item.kind === 'movie' ? 'film' : 'tv'} className="aspect-[4/3]" />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-black/70 to-transparent" />
      {showLogo ? (
        <img
          src={logo}
          alt=""
          onError={() => setLogoFailed(true)}
          className="pointer-events-none absolute inset-x-0 bottom-6 mx-auto max-h-[26%] max-w-[72%] object-contain drop-shadow-[0_2px_10px_rgb(0_0_0/0.6)]"
        />
      ) : (
        <p className="pointer-events-none absolute inset-x-4 bottom-6 line-clamp-2 text-center text-lg font-semibold leading-tight text-white drop-shadow-[0_2px_8px_rgb(0_0_0/0.7)]">{title}</p>
      )}
    </>
  );
}
