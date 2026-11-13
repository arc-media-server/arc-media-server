import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { ChevronRight, Folder, FolderSearch } from 'lucide-react';
import { api, type Library } from '@/lib/api';
import { Artwork } from '@/components/Artwork';
import { EmptyState, ErrorState } from '@/components/Page';
import { PosterGrid, PosterGridSkeleton } from '@/components/PosterCard';

/**
 * An Other videos library folder by folder: the folders in the current one (each with its oldest
 * video as a cover and how many videos it holds), then its own videos, oldest first. The folder
 * is in the address (?folder=), so Back and links work.
 */
export function VideoFolders({ library }: { library: Library }) {
  const { t } = useTranslation('library');
  const [params, setParams] = useSearchParams();
  const path = params.get('folder') ?? '';
  const query = useQuery({ queryKey: ['folders', library.id, path], queryFn: () => api.libraryFolders(library.id, path) });
  const open = (folder?: string) => {
    const next = new URLSearchParams(params);
    if (folder) next.set('folder', folder);
    else next.delete('folder');
    setParams(next);
    window.scrollTo(0, 0);
  };
  if (query.error) return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
  if (!query.data) return <PosterGridSkeleton />;
  const { folders, items, trail } = query.data;
  const href = (folder?: string) => {
    const next = new URLSearchParams(params);
    if (folder) next.set('folder', folder);
    else next.delete('folder');
    return `?${next}`;
  };
  return (
    <div className="space-y-8">
      {path && (
        <nav aria-label={t('library:folders.trail')} className="flex flex-wrap items-center gap-1 text-sm text-muted">
          <Link to={href()} onClick={(e) => { e.preventDefault(); open(); }} className="hover:text-foreground">{library.name}</Link>
          {trail.map((c, i) => (
            <span key={c.path} className="flex items-center gap-1">
              <ChevronRight className="size-3.5 text-subtle" />
              {i === trail.length - 1 ? (
                <span aria-current="page" className="font-medium text-foreground">{c.name}</span>
              ) : (
                <Link to={href(c.path)} onClick={(e) => { e.preventDefault(); open(c.path); }} className="hover:text-foreground">{c.name}</Link>
              )}
            </span>
          ))}
        </nav>
      )}
      {folders.length === 0 && items.length === 0 && (
        <EmptyState icon={<FolderSearch />} title={t('library:empty.title')}>
          {library.scan.running ? t('library:empty.scanning') : t('library:empty.paths', { paths: library.paths.join(', ') })}
        </EmptyState>
      )}
      {folders.length > 0 && (
        <div className="grid video-folder-grid gap-x-5 gap-y-7">
          {folders.map((f) => (
            <Link key={f.path} to={href(f.path)} onClick={(e) => { e.preventDefault(); open(f.path); }} className="group block outline-none" aria-label={t('library:folders.label', { name: f.name, count: f.count })}>
              <div className="relative overflow-hidden rounded-xl ring-1 ring-border transition group-hover:ring-foreground/30 group-focus-visible:ring-2 group-focus-visible:ring-accent">
                <Artwork src={f.cover} alt="" kind="still" fallbackTitle={f.name} />
                <span className="absolute left-2 top-2 flex items-center gap-1 rounded-md bg-black/60 px-1.5 py-0.5 text-[11px] font-medium text-white">
                  <Folder className="size-3" /> {t('library:folders.tag')}
                </span>
              </div>
              <p className="mt-2 truncate text-sm font-medium">{f.name}</p>
              <p className="mt-0.5 text-xs text-subtle">{t('library:counts.videos', { count: f.count })}</p>
            </Link>
          ))}
        </div>
      )}
      {items.length > 0 && <PosterGrid items={items} landscape />}
    </div>
  );
}
