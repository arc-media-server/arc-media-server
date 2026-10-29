import { useTranslation } from 'react-i18next';
import { createElement, useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Link, NavLink, useLocation, useNavigate, useSearchParams } from 'react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Bookmark, ChevronsUpDown, Sparkles, Tv, Home, ListVideo, Loader2, LogOut, Search, Settings, SlidersHorizontal, Users } from 'lucide-react';
import { api } from '@/lib/api';
import { useAuth, useIsAdmin } from '@/lib/auth';
import { usePrefs } from '@/lib/home';
import { navigationLibraries } from '@/lib/library-navigation';
import { isSettingsPath } from '@/lib/settings-navigation';
import { cn } from '@/lib/utils';
import { kindIcon } from '@/lib/kinds';
import { Avatar } from '@/components/Avatar';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { usePlaybackQueue } from './Player';
import { ActivityButton } from '@/components/Activity';
import { LibraryMenu } from '@/components/LibraryMenu';
import { LibraryNavigationSettings } from '@/components/LibraryNavigationSettings';
import { NewsButton } from '@/components/News';

export function useLibraries() {
  return useQuery({
    queryKey: ['libraries'],
    queryFn: api.libraries,
    // Poll while a scan is running so progress shows live.
    refetchInterval: (q) => (q.state.data?.some((l) => l.scan.running) ? 1500 : false),
  });
}

export function ArcLogo({ className }: { className?: string }) {
  // Each logo needs its own gradient id: a gradient defined inside a hidden element (the desktop
  // sidebar on phones) doesn't render for other SVGs that reference it.
  const id = useId();
  return (
    <svg viewBox="0 0 32 32" className={className} aria-hidden>
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#c4b5fd" />
          <stop offset="1" stopColor="#7c68ff" />
        </linearGradient>
      </defs>
      <path d="M5 24a11 11 0 0 1 22 0" fill="none" stroke={`url(#${id})`} strokeWidth="4.5" strokeLinecap="round" />
      <circle cx="16" cy="24" r="2.6" fill={`url(#${id})`} />
    </svg>
  );
}

function SearchBox({ className }: { className?: string }) {
  const { t } = useTranslation('common');
  const navigate = useNavigate();
  const location = useLocation();
  const [params] = useSearchParams();
  const [value, setValue] = useState(location.pathname === '/search' ? (params.get('q') ?? '') : '');
  const timer = useRef<number | undefined>(undefined);

  // Navigation (including Back/Forward) owns the query; discard pending typing on navigation.
  useEffect(() => {
    setValue(location.pathname === '/search' ? new URLSearchParams(location.search).get('q') ?? '' : '');
    window.clearTimeout(timer.current);
    return () => window.clearTimeout(timer.current);
  }, [location.pathname, location.search]);

  return (
    <label className={cn('flex h-9 min-w-0 items-center gap-2 rounded-xl bg-surface-2/80 px-3 text-sm ring-1 ring-border transition-shadow focus-within:ring-accent', className)}>
      <Search className="size-4 shrink-0 text-subtle" />
      <input
        value={value}
        onChange={(e) => {
          const next = e.target.value;
          setValue(next);
          window.clearTimeout(timer.current);
          // Replace while typing so Back leaves search in one step; clearing clears results too.
          timer.current = window.setTimeout(() => {
            const q = next.trim();
            if (q || location.pathname === '/search') navigate(q ? `/search?q=${encodeURIComponent(q)}` : '/search', { replace: location.pathname === '/search' });
          }, 150);
        }}
        placeholder={t('navigation.search')}
        className="min-w-0 w-full bg-transparent outline-none placeholder:text-subtle"
      />
    </label>
  );
}

function NavItem({ to, icon, children, busy, active }: { to: string; icon: ReactNode; children: ReactNode; busy?: boolean; active?: boolean }) {
  const { pathname } = useLocation();
  const selected = active ?? pathname === to;
  return (
    <Link to={to} aria-current={selected ? 'page' : undefined} className={cn(
      'flex h-9 items-center gap-3 rounded-lg px-2.5 text-[13.5px] font-medium transition-colors [&_svg]:size-[18px]',
      selected ? 'bg-surface-3 text-foreground' : 'text-muted hover:bg-surface-2 hover:text-foreground',
    )}>
      <span className="shrink-0">{icon}</span>
      <span className="flex-1 truncate">{children}</span>
      {busy && <Loader2 className="animate-spin text-accent" />}
    </Link>
  );
}

/** The current profile, with Switch Profile and Sign Out. */
function ProfileMenu({ compact }: { compact?: boolean }) {
  const { t } = useTranslation('common');
  const playback = usePlaybackQueue();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  // A deliberate profile change must not carry the old profile's browse query
  // into the new one. Initial sign-in deep links remain untouched.
  const clearBrowseQuery = () => {
    if (/^\/library\/\d+$/.test(location.pathname)) navigate(location.pathname, { replace: true });
  };
  const { data: status } = useAuth();
  const profile = status?.profile;
  if (!profile) return null;
  const leave = async () => {
    await playback.stop();
    await api.leaveProfile();
    clearBrowseQuery();
    qc.resetQueries();
  };
  const signOut = async () => {
    await playback.stop();
    await api.logout();
    clearBrowseQuery();
    qc.resetQueries();
  };
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        {compact ? (
          <button aria-label={t('profile.named', { name: profile.name })} className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-accent">
            <Avatar name={profile.name} color={profile.color} className="size-8 text-sm" />
          </button>
        ) : (
          <button className="flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-left transition-colors hover:bg-surface-2">
            <Avatar name={profile.name} color={profile.color} className="size-8 text-sm" />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-[13.5px] font-medium">{profile.name}</span>
              {profile.is_admin && <span className="block text-[11px] text-subtle">{t('profile.admin')}</span>}
            </span>
            <ChevronsUpDown className="size-4 text-subtle" />
          </button>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align={compact ? 'end' : 'start'} side={compact ? 'bottom' : 'top'} className="w-60">
        <DropdownMenuItem onSelect={() => navigate('/?edit=1')}>
          <SlidersHorizontal className="size-4 text-muted" />{' '}{t('profile.editHome')}</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => navigate('/year')}>
          <Sparkles className="size-4 text-muted" />{' '}{t('profile.yearInReview')}</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => navigate('/link')}>
          <Tv className="size-4 text-muted" />{' '}{t('profile.linkDevice')}</DropdownMenuItem>
        <DropdownMenuItem onSelect={() => navigate('/settings')}>
          <Settings className="size-4 text-muted" />{' '}{t('navigation.settings')}</DropdownMenuItem>
        <DropdownMenuSeparator />
        {!status.friend && (status.profiles.length > 1 || !status.password_session) && (
          <DropdownMenuItem onSelect={leave}>
            <Users className="size-4 text-muted" />{' '}{t('profile.switchProfile')}</DropdownMenuItem>
        )}
        {(status.password_session || status.friend) && (
          <DropdownMenuItem onSelect={signOut}>
            <LogOut className="size-4 text-muted" />{' '}{t('profile.signOut')}</DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function Sidebar() {
  const { t } = useTranslation('common');
  const { data } = useLibraries();
  const prefs = usePrefs();
  const libraries = navigationLibraries(data ?? [], prefs.data?.library_order);
  const admin = useIsAdmin();
  const { pathname } = useLocation();

  return (
    <aside className="glass fixed inset-y-0 left-0 z-40 hidden w-[248px] flex-col border-r border-border md:flex">
      <div className="flex h-16 items-center gap-2.5 px-5">
        <ArcLogo className="size-7" />
        <span className="text-[19px] font-bold tracking-tight">Arc</span>
        <div className="-mr-1.5 ml-auto flex items-center">
          <NewsButton />
          <ActivityButton />
        </div>
      </div>
      <div className="px-3 pb-3">
        <SearchBox />
      </div>
      <nav className="flex-1 space-y-6 overflow-y-auto px-3 pb-6">
        <div className="space-y-0.5">
          <NavItem to="/" icon={<Home />}>{t('navigation.home')}</NavItem>
          <NavItem to="/watchlist" icon={<Bookmark />}>{t('navigation.watchlist')}</NavItem>
          <NavItem to="/playlists" icon={<ListVideo />}>{t('navigation.playlists')}</NavItem>
        </div>
        {!!libraries?.length && (
          <div>
            <div className="flex items-center justify-between"><p className="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-wider text-subtle">{t('navigation.libraries')}</p><LibraryNavigationSettings /></div>
            <div className="space-y-0.5">
              {libraries.map((lib) => (
                <div key={lib.id} className="flex items-center [&>a]:min-w-0 [&>a]:flex-1">
                <NavItem
                  to={`/library/${lib.id}`}
                  icon={createElement(kindIcon(lib.kind))}
                  busy={lib.scan.running}
                >
                  {lib.name}
                </NavItem>
                {admin && <LibraryMenu lib={lib} compact />}
                </div>
              ))}
            </div>
          </div>
        )}
      </nav>
      <div className="space-y-1 border-t border-border p-2">
        <NavItem to="/settings" icon={<Settings />} active={isSettingsPath(pathname)}>{t('navigation.settings')}</NavItem>
        <ProfileMenu />
      </div>
    </aside>
  );
}

/** Compact top bar used instead of the sidebar on small screens. */
export function MobileBar() {
  const { t } = useTranslation('common');
  const { data } = useLibraries();
  const prefs = usePrefs();
  const libraries = navigationLibraries(data ?? [], prefs.data?.library_order);
  const { pathname } = useLocation();
  const link = ({ isActive }: { isActive: boolean }) =>
    cn('shrink-0 rounded-full px-3.5 py-1.5 text-[13px] font-medium transition-colors', isActive ? 'bg-foreground text-background' : 'bg-surface-2 text-muted');
  return (
    <div className="glass relative z-40 border-b border-border md:hidden">
      <div className="flex items-center gap-3 px-4 pt-3">
        <ArcLogo className="size-6" />
        <SearchBox className="flex-1" />
        <NewsButton />
        <ActivityButton />
        <Link to="/settings" aria-label={t('navigation.settings')} aria-current={isSettingsPath(pathname) ? 'page' : undefined} className={cn('flex size-8 shrink-0 items-center justify-center rounded-full text-muted hover:bg-surface-2 hover:text-foreground', isSettingsPath(pathname) && 'bg-surface-3 text-foreground')}><Settings className="size-[18px]" /></Link>
        <LibraryNavigationSettings />
        <ProfileMenu compact />
      </div>
      <div className="flex min-w-0 items-center">
      <nav className="scrollbar-none flex min-w-0 flex-1 gap-2 overflow-x-auto px-4 py-3">
        <NavLink to="/" end className={link}>{t('navigation.home')}</NavLink>
        <NavLink to="/watchlist" className={link}>{t('navigation.watchlist')}</NavLink>
        <NavLink to="/playlists" className={link}>{t('navigation.playlists')}</NavLink>
        {libraries?.map((lib) => (
          <NavLink key={lib.id} to={`/library/${lib.id}`} className={link}>
            {lib.name}
          </NavLink>
        ))}
      </nav>
      </div>
    </div>
  );
}

