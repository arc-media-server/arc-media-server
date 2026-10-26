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
  const [open, setOpen] = useState(false);
  const hidden = useSpoilerGuard()(episode);
  const detail = useQuery({ queryKey: ['item', episode.id], queryFn: () => api.item(episode.id), enabled: open && !hidden, staleTime: 0 });
  return <Dialog open={open} onOpenChange={setOpen}>
    <Button variant="ghost" size="icon" title="Episode details" aria-label={`Details for ${episode.title}`} onClick={() => setOpen(true)}><BookOpen /></Button>
    <DialogContent title={episode.title} description="Episode details">
      {hidden ? <p className="py-4 text-sm text-muted">Description and guest appearances hidden until you watch it.</p> : <>
        <ErrorNotice error={detail.error} onRetry={() => void detail.refetch()} />
        {!detail.data && !detail.error && <p role="status">Loading episode details…</p>}
        {detail.data && <>
          {detail.data.overview && <p className="text-sm leading-relaxed text-muted">{detail.data.overview}</p>}
          <CrewLine item={detail.data} />
          <div className="-mx-6"><CastRow credits={detail.data.credits} title="Guest cast" /></div>
          {!detail.data.credits?.length && <p className="mt-5 text-sm text-subtle">No cast or crew supplied by the file episode order provider.</p>}
        </>}
      </>}
    </DialogContent>
  </Dialog>;
}
