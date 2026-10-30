import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Eye, EyeOff, SlidersHorizontal } from 'lucide-react';
import { api, type Library, type RecommendedLook } from '@/lib/api';
import { ReorderList } from './ArrangeRow';
import { Button } from './ui/button';
import { Dialog, DialogContent } from './ui/dialog';
import { inputClass } from './ui/field';
import { ErrorNotice } from './Page';

const families: RecommendedLook['type'][] = ['continue', 'recently_released', 'recently_added', 'start_watching', 'because_you_watched', 'top_rated', 'recently_watched', 'rediscover', 'more_in_genre', 'top_in_genre', 'top_by_person', 'top_unwatched'];
function complete(saved: RecommendedLook[], shows: boolean): RecommendedLook[] {
  return [...saved.filter(r => families.includes(r.type)), ...families.filter(type => !saved.some(r => r.type === type)).map((type): RecommendedLook => ({ type }))]
    .filter(r => shows || (r.type !== 'start_watching' && r.type !== 'recently_watched'));
}

export function EditRecommended({ library }: { library: Library }) {
  const { t } = useTranslation('library');
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<RecommendedLook[]>();
  const layout = useQuery({ queryKey: ['recommended-layout', library.id], queryFn: () => api.recommendedLayout(library.id), enabled: open });
  const rows = draft ?? complete(layout.data?.rows ?? [], library.kind === 'shows');
  const save = useMutation({ mutationFn: () => api.saveRecommendedLayout(library.id, draft ?? rows), onSuccess: data => {
    qc.setQueryData(['recommended-layout', library.id], data);
    void qc.invalidateQueries({ queryKey: ['recommended', library.id] });
    setOpen(false);
  } });
  const names: Record<RecommendedLook['type'], string> = {
    continue: t('recommendedEditor.continue'), recently_released: t('recommendedEditor.recentlyReleased'), recently_added: t('recommendedEditor.recentlyAdded'), start_watching: t('recommendedEditor.startWatching'), because_you_watched: t('recommendedEditor.becauseYouWatched'), top_rated: t('recommendedEditor.topRated'), recently_watched: t('recommendedEditor.recentlyWatched'), rediscover: t('recommendedEditor.rediscover'), more_in_genre: t('recommendedEditor.moreInGenre'), top_in_genre: t('recommendedEditor.topInGenre'), top_by_person: t('recommendedEditor.topByPerson'), top_unwatched: t('recommendedEditor.topUnwatched'),
  };
  const change = (index: number, patch: Partial<RecommendedLook>) => setDraft(rows.map((r, i) => i === index ? { ...r, ...patch } : r));
  return <>
    <Button variant="secondary" size="sm" onClick={() => { setDraft(undefined); save.reset(); setOpen(true); }}><SlidersHorizontal />{t('recommendedEditor.title')}</Button>
    <Dialog open={open} onOpenChange={value => { if (!save.isPending) setOpen(value); }}>
      <DialogContent title={t('recommendedEditor.title')} description={t('recommendedEditor.description', { name: library.name })} footer={<>
        <Button variant="ghost" disabled={save.isPending} onClick={() => setDraft(complete([], library.kind === 'shows'))}>{t('recommendedEditor.reset')}</Button>
        <Button variant="ghost" disabled={save.isPending} onClick={() => setOpen(false)}>{t('recommendedEditor.cancel')}</Button>
        <Button variant="primary" disabled={save.isPending || layout.isPending || !!layout.error} onClick={() => save.mutate()}>{t('recommendedEditor.save')}</Button>
      </>}>
        <ErrorNotice error={layout.error} onRetry={() => void layout.refetch()} />
        <ErrorNotice error={save.error} />
        {!layout.isPending && !layout.error && <fieldset disabled={save.isPending}><ReorderList items={rows} keyOf={r => r.type} titleOf={r => names[r.type]} onReorder={setDraft} disabled={save.isPending} className="space-y-2">
          {(r, index) => <>
            <span className={'min-w-0 flex-1 break-words text-sm ' + (r.hidden ? 'opacity-45' : '')}>{names[r.type]}</span>
            <select aria-label={t('recommendedEditor.style', { title: names[r.type] })} value={r.style ?? ''} onChange={e => change(index, { style: e.target.value as RecommendedLook['style'] })} className={inputClass + ' max-w-28 text-xs'}>
              <option value="">{t('recommendedEditor.automatic')}</option><option value="poster">{t('home:look.poster')}</option><option value="thumb">{t('home:look.thumb')}</option><option value="arc">{t('home:look.arc')}</option>
            </select>
            <Button size="icon" variant="ghost" aria-label={r.hidden ? t('home:rowActions.show', { title: names[r.type] }) : t('home:rowActions.hide', { title: names[r.type] })} onClick={() => change(index, { hidden: !r.hidden })}>{r.hidden ? <EyeOff /> : <Eye />}</Button>
          </>}
        </ReorderList></fieldset>}
      </DialogContent>
    </Dialog>
  </>;
}
