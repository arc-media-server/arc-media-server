import { useTranslation } from 'react-i18next';
import { MoreHorizontal, X } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type Item } from '@/lib/api';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

/** Takes something out of Continue Watching (and Up Next) without marking it watched. */
export function useRemoveFromContinue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (item: Item) => api.removeFromContinue(item.id),
    onSuccess: () => {
      for (const key of ['continue', 'next-up']) qc.invalidateQueries({ queryKey: [key] });
    },
  });
}

/** The ⋯ button in a card's corner, shown on hover, with "Remove from Continue Watching". */
export function ContinueCardMenu({ item, onRemove }: { item: Item; onRemove: (item: Item) => void }) {
  const { t } = useTranslation('titles');
  return (
    <div className="absolute left-2 top-2 z-10" onClick={(e) => e.stopPropagation()}>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label={t('titles:cardMenu.moreTitle', { title: item.series_title ?? item.title })}
            className="flex size-8 items-center justify-center rounded-full bg-black/60 text-white opacity-100 md:opacity-0 shadow-lg backdrop-blur transition-opacity hover:bg-black/80 focus-visible:opacity-100 group-hover:opacity-100 data-[state=open]:opacity-100"
          >
            <MoreHorizontal className="size-4" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem onSelect={() => onRemove(item)}>
            <X className="size-4 text-muted" /> {t("titles:cardMenu.removeFromContinueWatching")}</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
