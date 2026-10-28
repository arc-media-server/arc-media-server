import { useTranslation } from 'react-i18next';
import { Clapperboard, Film, Play } from 'lucide-react';
import type { Extra } from '@/lib/api';
import * as fmt from '@/lib/format';
import { usePlayer } from './Player';
import { RowShell } from './RowShell';
import { Button } from './ui/button';



/** The hero's "Trailer" button, when there's a trailer. */
export function TrailerButton({ extras, title }: { extras?: Extra[]; title: string }) {
  const { t } = useTranslation('titles');
  const play = usePlayer();
  const trailer = extras?.find((e) => e.kind === 'trailer');
  if (!trailer) return null;
  return (
    <Button variant="secondary" size="lg" className="size-12 rounded-2xl px-0" title={t("titles:extras.playTheTrailer")} aria-label={t("titles:extras.trailer")}
      onClick={() => play({ title: trailer.title, subtitle: t('titles:extras.trailerSubtitle', { title }), src: trailer.stream_url })}>
      <Clapperboard />
    </Button>
  );
}

/** A row of a movie or show's extras. */
export function ExtrasRow({ extras, title }: { extras?: Extra[]; title: string }) {
  const { t } = useTranslation('titles');
  const kindNames: Record<Extra['kind'], string> = {
    trailer: t('titles:extras.trailer'),
    featurette: t('titles:extras.featurette'),
    behind_the_scenes: t('titles:extras.behindTheScenes'),
    deleted_scene: t('titles:extras.deletedScene'),
    interview: t('titles:extras.interview'),
    scene: t('titles:extras.scene'),
    short: t('titles:extras.short'),
    other: t('titles:extras.other'),
  };
  const play = usePlayer();
  if (!extras?.length) return null;
  return (
    <RowShell title={t("titles:extras.extras")} className="mt-10">
        {extras.map((e) => (
          <button
            key={e.id}
            type="button"
            onClick={() => play({ title: e.title, subtitle: t('titles:extras.subtitle', { title, kind: kindNames[e.kind] }), src: e.stream_url })}
            className="group w-60 shrink-0 snap-start text-left outline-none"
          >
            <div className="relative flex aspect-video items-center justify-center overflow-hidden rounded-xl bg-gradient-to-br from-surface-2 to-surface-3 ring-1 ring-border transition-all duration-300 group-hover:-translate-y-1 group-focus-visible:ring-2 group-focus-visible:ring-accent">
              <Film className="size-7 text-subtle transition-opacity group-hover:opacity-0" strokeWidth={1.5} />
              <span className="absolute flex size-11 scale-90 items-center justify-center rounded-full bg-white/90 text-black opacity-0 shadow-xl transition-all duration-300 group-hover:scale-100 group-hover:opacity-100">
                <Play className="ml-0.5 size-5 fill-current" />
              </span>
              {e.duration_ms ? (
                <span className="absolute bottom-2 right-2 rounded-md bg-black/60 px-1.5 py-0.5 text-[11px] font-medium text-white">
                  {fmt.clock(e.duration_ms)}
                </span>
              ) : null}
            </div>
            <p className="mt-2.5 truncate text-[13px] font-medium">{e.title}</p>
            <p className="text-xs text-subtle">{kindNames[e.kind]}</p>
          </button>
        ))}
    </RowShell>
  );
}
