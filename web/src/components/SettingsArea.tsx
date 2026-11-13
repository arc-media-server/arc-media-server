import { useTranslation } from 'react-i18next';
import { useEffect } from 'react';
import { Link, Navigate, Outlet, useLocation, useNavigate } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useIsAdmin } from '@/lib/auth';
import { selectedSettingsSection, settingsNavigation } from '@/lib/settings-navigation';
import { cn } from '@/lib/utils';
import { inputClass } from '@/components/ui/field';

/** Shared settings navigation; dedicated tools keep their URLs and full page content. */
export function SettingsArea() {
  const { t } = useTranslation('common');
  const navigation = settingsNavigation();
  const admin = useIsAdmin();
  const location = useLocation();
  const navigate = useNavigate();
  const section = selectedSettingsSection(new URLSearchParams(location.search).get('section'));
  const selected = location.pathname === '/settings' ? section : location.pathname.slice(1);
  const health = useQuery({
    queryKey: ['health-summary'], enabled: admin,
    queryFn: async () => Object.values(await api.healthSummary()).reduce((a, b) => a + b, 0),
  });
  useEffect(() => { window.scrollTo(0, 0); }, [selected]);

  // Avoid mounting management tools or issuing their queries for ordinary profiles.
  if (!admin) return location.pathname === '/settings' ? <Outlet /> : <Navigate to="/settings" replace />;
  return (
    <div className="lg:grid lg:grid-cols-[248px_minmax(0,1fr)] lg:items-start">
      <nav aria-label={t('settings.sectionsLabel')} className="sticky top-0 z-30 bg-background/95 px-6 py-3 backdrop-blur-xl md:px-10 lg:top-6 lg:ml-6 lg:mt-10 lg:max-h-[calc(100dvh-48px)] lg:overflow-y-auto lg:rounded-2xl lg:bg-surface lg:p-3">
        <label className="sr-only" htmlFor="settings-section">{t('settings.section')}</label>
        <select id="settings-section" value={selected} onChange={e => {
          const link = navigation.flatMap(g => g.links).find(l => l.id === e.target.value);
          if (link) navigate(link.to);
        }} className={inputClass + ' w-full lg:hidden'}>
          {navigation.map(group => <optgroup key={group.label} label={group.label}>
            {group.links.map(link => <option key={link.id} value={link.id}>{link.label}{link.id === 'health' && health.data ? ` (${health.data})` : ''}</option>)}
          </optgroup>)}
        </select>
        <div className="hidden lg:block">
          <p className="mb-4 px-3 text-lg font-semibold">{t('navigation.settings')}</p>
          {navigation.map(group => <div key={group.label} className="mb-4 border-t border-border pt-4 first:border-0 first:pt-0 last:mb-0">
            <p className="mb-1 px-3 text-[11px] font-semibold uppercase tracking-wider text-subtle">{group.label}</p>
            {group.links.map(link => <Link key={link.id} to={link.to} aria-current={selected === link.id ? 'page' : undefined} className={cn('flex min-h-10 items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium', selected === link.id ? 'bg-foreground text-background' : 'text-muted hover:bg-surface-2 hover:text-foreground')}>
              <span className="min-w-0 flex-1">{link.label}</span>
              {link.id === 'health' && !!health.data && <span aria-label={t('settings.libraryIssues', { count: health.data })} className="shrink-0 rounded-full bg-warning/15 px-1.5 text-[11px] font-semibold text-warning">{health.data}</span>}
            </Link>)}
          </div>)}

        </div>
      </nav>
      <div className="min-w-0"><Outlet /></div>
    </div>
  );
}
