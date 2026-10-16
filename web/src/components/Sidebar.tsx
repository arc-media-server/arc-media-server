import { useEffect, useId, useState, type ReactNode } from 'react';
import { NavLink, useLocation, useNavigate, useSearchParams } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, DatabaseBackup, Film, Home, Loader2, Search, Tv } from 'lucide-react';
import { api, type Library } from '@/lib/api';
import { cn } from '@/lib/utils';

export function useLibraries() {
  return useQuery({
    queryKey: ['libraries'],
    queryFn: api.libraries,
    // Poll while a scan is running so progress shows live.
    refetchInterval: (q) => (q.state.data?.some((l) => l.scan.running) ? 1500 : false),
  });
}

export function useAttentionCount(libraries?: Library[]) {
  return useQuery({
    queryKey: ['attention', libraries?.map((l) => l.id)],
    enabled: !!libraries,
    queryFn: async () => {
      const lists = await Promise.all(libraries!.map((l) => api.unmatched(l.id)));
      return lists.reduce((n, l) => n + l.length, 0);
    },
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

export function Sidebar() {
  const { data: libraries } = useLibraries();
  const { data: attention } = useAttentionCount(libraries);

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
        <div>
          <p className="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-wider text-subtle">Manage</p>
          <div className="space-y-0.5">
            <NavItem to="/attention" icon={<AlertTriangle />} badge={attention}>
              Needs Attention
            </NavItem>
            <NavItem to="/backups" icon={<DatabaseBackup />}>
              Backups
            </NavItem>
          </div>
        </div>
      </nav>
    </aside>
  );
}

/** Compact top bar used instead of the sidebar on small screens. */
export function MobileBar() {
  const { data: libraries } = useLibraries();
  const link = ({ isActive }: { isActive: boolean }) =>
    cn('shrink-0 rounded-full px-3.5 py-1.5 text-[13px] font-medium transition-colors', isActive ? 'bg-foreground text-background' : 'bg-surface-2 text-muted');
  return (
    <div className="glass relative z-40 border-b border-border md:hidden">
      <div className="flex items-center gap-3 px-4 pt-3">
        <ArcLogo className="size-6" />
        <SearchBox className="flex-1" />
      </div>
      <nav className="scrollbar-none flex gap-2 overflow-x-auto px-4 py-3">
        <NavLink to="/" end className={link}>
          Home
        </NavLink>
        {libraries?.map((lib) => (
          <NavLink key={lib.id} to={`/library/${lib.id}`} className={link}>
            {lib.name}
          </NavLink>
        ))}
        <NavLink to="/attention" className={link}>
          Needs Attention
        </NavLink>
        <NavLink to="/backups" className={link}>
          Backups
        </NavLink>
      </nav>
    </div>
  );
}
