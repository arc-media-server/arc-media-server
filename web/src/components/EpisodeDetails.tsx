import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { BookOpen } from 'lucide-react';
import { api, type Item } from '@/lib/api';
import { useSpoilerGuard } from '@/lib/home';
import { CastRow, CrewLine } from '@/components/Credits';
import { ErrorNotice } from '@/components/Page';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';

/** Read details independently of Play, including unavailable episodes. */
export function EpisodeDetailsButton({ episode }: { episode: Item }) {
  const { t } = useTranslation('titles');
  const [open, setOpen] = useState(false);
  const hidden = useSpoilerGuard()(episode);
  const detail = useQuery({ queryKey: ['item', episode.id], queryFn: () => api.item(episode.id), enabled: open && !hidden, staleTime: 0 });
  return <Dialog open={open} onOpenChange={setOpen}>
    <Button variant="ghost" size="icon" title={t("titles:episodeDetails.episodeDetails")} aria-label={t('titles:episodeDetails.forTitle', { title: episode.title })} onClick={() => setOpen(true)}><BookOpen /></Button>
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
