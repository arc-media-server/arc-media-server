import { useEffect, useId, useState, type ReactNode } from 'react';
import { NavLink, useLocation, useNavigate, useSearchParams } from 'react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Activity, ChartColumn, ChevronsUpDown, DatabaseBackup, EyeOff, Sparkles, Film, FolderCog, HeartPulse, Home, ListVideo, Loader2, LogOut, Monitor, Moon, Search, Settings, SlidersHorizontal, Sun, Tv, Users } from 'lucide-react';
import { api, type Theme } from '@/lib/api';
import { useAuth, useIsAdmin } from '@/lib/auth';
import { cn } from '@/lib/utils';
import { Avatar } from '@/components/Avatar';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { usePrefs, useUpdatePrefs } from '@/lib/home';

export function useLibraries() {
  return useQuery({
    queryKey: ['libraries'],
    queryFn: api.libraries,
    // Poll while a scan is running so progress shows live.
    refetchInterval: (q) => (q.state.data?.some((l) => l.scan.running) ? 1500 : false),
  });
}

/** How many things across all libraries need the user (unidentified, unreadable, unrecognised). */
export function useHealthCount(enabled: boolean) {
  return useQuery({
    queryKey: ['health-summary'],
    enabled,
    queryFn: async () => Object.values(await api.healthSummary()).reduce((a, b) => a + b, 0),
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
  const navigate = useNavigate();
  const location = useLocation();
  const [params] = useSearchParams();
  const [value, setValue] = useState(location.pathname === '/search' ? (params.get('q') ?? '') : '');

  // Search as you type, replacing history entries so Back leaves search in one step.
  useEffect(() => {
    const t = window.setTimeout(() => {
      const q = value.trim();
      if (q) navigate(`/search?q=${encodeURIComponent(q)}`, { replace: location.pathname === '/search' });
    }, 250);
    return () => window.clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  return (
    <label className={cn('flex h-9 items-center gap-2 rounded-xl bg-surface-2/80 px-3 text-sm ring-1 ring-border transition-shadow focus-within:ring-accent', className)}>
      <Search className="size-4 shrink-0 text-subtle" />
      <input
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Search"
        className="w-full bg-transparent outline-none placeholder:text-subtle"
      />
    </label>
  );
}

function NavItem({ to, icon, children, badge, busy }: { to: string; icon: ReactNode; children: ReactNode; badge?: number; busy?: boolean }) {
  return (
    <NavLink
      to={to}
      end
      className={({ isActive }) =>
        cn(
          'flex h-9 items-center gap-3 rounded-lg px-2.5 text-[13.5px] font-medium transition-colors [&_svg]:size-[18px]',
          isActive ? 'bg-surface-3 text-foreground' : 'text-muted hover:bg-surface-2 hover:text-foreground',
        )
      }
    >
      <span className="shrink-0">{icon}</span>
      <span className="flex-1 truncate">{children}</span>
      {busy && <Loader2 className="animate-spin text-accent" />}
      {!!badge && (
        <span className="rounded-full bg-warning/15 px-1.5 text-[11px] font-semibold text-warning">{badge}</span>
      )}
    </NavLink>
  );
}

/** The current profile, with Switch Profile and Sign Out. */
function ProfileMenu({ compact }: { compact?: boolean }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const { data: status } = useAuth();
  const prefs = usePrefs();
  const updatePrefs = useUpdatePrefs();
  const profile = status?.profile;
  if (!profile) return null;
  const leave = async () => {
    await api.leaveProfile();
    qc.resetQueries();
  };
  const signOut = async () => {
    await api.logout();
    qc.resetQueries();
  };
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        {compact ? (
          <button aria-label={`Profile: ${profile.name}`} className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-accent">
            <Avatar name={profile.name} color={profile.color} className="size-8 text-sm" />
          </button>
        ) : (
          <button className="flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-left transition-colors hover:bg-surface-2">
            <Avatar name={profile.name} color={profile.color} className="size-8 text-sm" />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-[13.5px] font-medium">{profile.name}</span>
              {profile.is_admin && <span className="block text-[11px] text-subtle">Admin</span>}
            </span>
            <ChevronsUpDown className="size-4 text-subtle" />
          </button>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent align={compact ? 'end' : 'start'} side={compact ? 'bottom' : 'top'} className="w-60">
        <DropdownMenuCheckboxItem
          checked={!!prefs.data?.hide_spoilers}
          disabled={!prefs.data}
          onSelect={(e) => e.preventDefault()}
          onCheckedChange={(on) => updatePrefs.mutate({ hide_spoilers: on })}
        >
          <EyeOff className="size-4 shrink-0 self-start text-muted mt-0.5" />
          <span className="min-w-0">
            <span className="block">Hide Spoilers</span>
            <span className="block text-xs text-subtle">Blur stills and hide descriptions of episodes you haven't seen</span>
          </span>
        </DropdownMenuCheckboxItem>
        <DropdownMenuCheckboxItem
          checked={prefs.data?.auto_play !== false}
          disabled={!prefs.data}
          onSelect={(e) => e.preventDefault()}
          onCheckedChange={(on) => updatePrefs.mutate({ auto_play: on })}
        >
          <ListVideo className="size-4 shrink-0 self-start text-muted mt-0.5" />
          <span className="min-w-0">
            <span className="block">Auto-Play</span>
            <span className="block text-xs text-subtle">Play the next episode after a short countdown</span>
          </span>
        </DropdownMenuCheckboxItem>
        <DropdownMenuSeparator />
        <DropdownMenuLabel>Appearance</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={prefs.data?.theme ?? ''} onValueChange={(v) => updatePrefs.mutate({ theme: v as Theme })}>
          {themes.map((t) => (
            <DropdownMenuRadioItem key={t.value} value={t.value} disabled={!prefs.data} onSelect={(e) => e.preventDefault()}>
              {t.icon} {t.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => navigate('/?edit=1')}>
          <SlidersHorizontal className="size-4 text-muted" /> Edit Home…
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => navigate('/year')}>
          <Sparkles className="size-4 text-muted" /> Your Year in Review
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {(status.profiles.length > 1 || !status.password_session) && (
          <DropdownMenuItem onSelect={leave}>
            <Users className="size-4 text-muted" /> Switch Profile
          </DropdownMenuItem>
        )}
        {status.password_session && (
          <DropdownMenuItem onSelect={signOut}>
            <LogOut className="size-4 text-muted" /> Sign Out
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function Sidebar() {
  const { data: libraries } = useLibraries();
  const admin = useIsAdmin();
  const { data: problems } = useHealthCount(admin);

  return (
    <aside className="glass fixed inset-y-0 left-0 z-40 hidden w-[248px] flex-col border-r border-border md:flex">
      <div className="flex h-16 items-center gap-2.5 px-5">
        <ArcLogo className="size-7" />
        <span className="text-[19px] font-bold tracking-tight">Arc</span>
      </div>
      <div className="px-3 pb-3">
        <SearchBox />
      </div>
      <nav className="flex-1 space-y-6 overflow-y-auto px-3 pb-6">
        <div className="space-y-0.5">
          <NavItem to="/" icon={<Home />}>
            Home
          </NavItem>
          <NavItem to="/playlists" icon={<ListVideo />}>
            Playlists
          </NavItem>
        </div>
        {!!libraries?.length && (
          <div>
            <p className="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-wider text-subtle">Libraries</p>
            <div className="space-y-0.5">
              {libraries.map((lib) => (
                <NavItem
                  key={lib.id}
                  to={`/library/${lib.id}`}
                  icon={lib.kind === 'shows' ? <Tv /> : <Film />}
                  busy={lib.scan.running}
                >
                  {lib.name}
                </NavItem>
              ))}
            </div>
          </div>
        )}
        {admin && (
          <div>
            <p className="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-wider text-subtle">Manage</p>
            <div className="space-y-0.5">
              {manageLinks.map((l) => (
                <NavItem key={l.to} to={l.to} icon={l.icon} badge={l.to === '/health' ? problems : undefined}>
                  {l.label}
                </NavItem>
              ))}
            </div>
          </div>
        )}
      </nav>
      <div className="border-t border-border p-2">
        <ProfileMenu />
      </div>
    </aside>
  );
}

const manageLinks = [
  { to: '/dashboard', label: 'Dashboard', icon: <Activity /> },
  { to: '/insights', label: 'Insights', icon: <ChartColumn /> },
  { to: '/health', label: 'Library Health', icon: <HeartPulse /> },
  { to: '/libraries', label: 'Libraries', icon: <FolderCog /> },
  { to: '/profiles', label: 'Profiles', icon: <Users /> },
  { to: '/backups', label: 'Backups', icon: <DatabaseBackup /> },
  { to: '/settings', label: 'Settings', icon: <Settings /> },
];

/** Compact top bar used instead of the sidebar on small screens. */
export function MobileBar() {
  const { data: libraries } = useLibraries();
  const admin = useIsAdmin();
  const link = ({ isActive }: { isActive: boolean }) =>
    cn('shrink-0 rounded-full px-3.5 py-1.5 text-[13px] font-medium transition-colors', isActive ? 'bg-foreground text-background' : 'bg-surface-2 text-muted');
  return (
    <div className="glass relative z-40 border-b border-border md:hidden">
      <div className="flex items-center gap-3 px-4 pt-3">
        <ArcLogo className="size-6" />
        <SearchBox className="flex-1" />
        <ProfileMenu compact />
      </div>
      <nav className="scrollbar-none flex gap-2 overflow-x-auto px-4 py-3">
        <NavLink to="/" end className={link}>
          Home
        </NavLink>
        <NavLink to="/playlists" className={link}>
          Playlists
        </NavLink>
        {libraries?.map((lib) => (
          <NavLink key={lib.id} to={`/library/${lib.id}`} className={link}>
            {lib.name}
          </NavLink>
        ))}
        {admin &&
          manageLinks.map((l) => (
            <NavLink key={l.to} to={l.to} className={link}>
              {l.label}
            </NavLink>
          ))}
      </nav>
    </div>
  );
}

const themes: { value: Theme; label: string; icon: ReactNode }[] = [
  { value: '', label: 'Automatic', icon: <Monitor className="size-4 text-muted" /> },
  { value: 'dark', label: 'Dark', icon: <Moon className="size-4 text-muted" /> },
  { value: 'light', label: 'Light', icon: <Sun className="size-4 text-muted" /> },
];
