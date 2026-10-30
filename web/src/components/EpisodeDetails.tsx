import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { api, type Item } from '@/lib/api';
import { useSpoilerGuard } from '@/lib/home';
import { CastRow, CrewLine } from '@/components/Credits';
import { ErrorNotice } from '@/components/Page';
import { Dialog, DialogContent } from '@/components/ui/dialog';

/** Read details independently of Play, including unavailable episodes; opened from the episode's "⋯". */
export function EpisodeDetailsDialog({ episode, open, onOpenChange }: { episode: Item; open: boolean; onOpenChange: (o: boolean) => void }) {
  const { t } = useTranslation('titles');
  const hidden = useSpoilerGuard()(episode);
  const detail = useQuery({ queryKey: ['item', episode.id], queryFn: () => api.item(episode.id), enabled: open && !hidden, staleTime: 0 });
  return <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent title={episode.title} description={t("titles:episodeDetails.episodeDetails")}>
      {hidden ? <p className="py-4 text-sm text-muted">{t("titles:episodeDetails.descriptionAndGuestAppearances")}</p> : <>
        <ErrorNotice error={detail.error} onRetry={() => void detail.refetch()} />
        {!detail.data && !detail.error && <p role="status">{t("titles:episodeDetails.loadingEpisodeDetails")}</p>}
        {detail.data && <>
          {detail.data.overview && <p className="text-sm leading-relaxed text-muted">{detail.data.overview}</p>}
          <CrewLine item={detail.data} />
          <div className="-mx-6"><CastRow credits={detail.data.credits} title={t("titles:episodeDetails.guestCast")} /></div>
          {!detail.data.credits?.length && <p className="mt-5 text-sm text-subtle">{t("titles:episodeDetails.noCastOrCrew")}</p>}
        </>}
      </>}
    </DialogContent>
  </Dialog>;
}
