import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Check, Copy, Crown, Loader2, LogOut, Users, X } from 'lucide-react';
import { api, type Item } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { formatCode, normalizeCode, RoomError, RoomSession, rooms, type WatchParticipant, type WatchRoom } from '@/lib/watch-together';
import { Button } from './ui/button';
import { Dialog, DialogContent } from './ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu';
import { cn } from '@/lib/utils';

interface Invite {
  code: string;
  expiresAt: number;
}

interface WatchTogether {
  /** This server has Watch Together and the profile may use it. */
  available: boolean;
  session: RoomSession | null;
  room: WatchRoom | null;
  /** The invite code, for the device that created or renewed it (Arc shows codes only then). */
  invite: Invite | null;
  /** Why the last room ended for this device, until something new starts. */
  ended: 'ended' | 'gone' | null;
  start: (item: Item) => Promise<void>;
  join: (code: string) => Promise<void>;
  leave: () => Promise<void>;
  end: () => Promise<void>;
  renew: () => Promise<void>;
  /** Opens the dialog for typing a code. */
  openJoin: () => void;
}

const Context = createContext<WatchTogether>({
  available: false, session: null, room: null, invite: null, ended: null,
  start: async () => {}, join: async () => {}, leave: async () => {}, end: async () => {}, renew: async () => {}, openJoin: () => {},
});

export const useWatchTogether = () => useContext(Context);

/**
 * This device's Watch Together room (docs/WATCH-TOGETHER-API.md). The player follows it: when a
 * room is active it plays the room's title, and Play, Pause and seeking go to the room.
 */
export function WatchTogetherProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation(['player', 'common']);
  const profileId = useAuth().data?.profile?.id;
  const capabilities = useQuery({ queryKey: ['capabilities', profileId], queryFn: api.capabilities, enabled: !!profileId, staleTime: Infinity, retry: false });
  const feature = capabilities.data?.features.watch_together;
  const available = !!(feature?.available && feature.allowed);
  const [session, setSession] = useState<RoomSession | null>(null);
  const [room, setRoom] = useState<WatchRoom | null>(null);
  const [invite, setInvite] = useState<Invite | null>(null);
  const [ended, setEnded] = useState<'ended' | 'gone' | null>(null);
  const [joining, setJoining] = useState(false);
  const [recovered, setRecovered] = useState<WatchRoom | null>(null);
  const [startFailed, setStartFailed] = useState(false);
  const sessionRef = useRef<RoomSession | null>(null);

  const begin = useCallback((r: WatchRoom) => {
    sessionRef.current?.stop();
    setEnded(null);
    setRecovered(null);
    const s = new RoomSession(r, setRoom, (why) => {
      if (sessionRef.current !== s) return;
      sessionRef.current = null;
      setSession(null);
      setRoom(null);
      setInvite(null);
      setEnded(why);
    });
    sessionRef.current = s;
    setSession(s);
    setRoom(r);
    s.start();
  }, []);

  // A reload (or another tab closing) keeps the membership on the server: offer to carry on.
  useEffect(() => {
    if (!available || sessionRef.current) return;
    let cancelled = false;
    rooms.current().then((r) => { if (!cancelled && r && r.state !== 'ended') setRecovered(r); }).catch(() => undefined);
    return () => { cancelled = true; };
  }, [available, profileId]);

  // Switching profiles or signing out leaves the sync behind (the server drops it after 30 s).
  useEffect(() => () => { sessionRef.current?.stop(); sessionRef.current = null; }, [profileId]);

  const value: WatchTogether = {
    available, session, room, invite, ended,
    start: async (item) => {
      setStartFailed(false);
      const created = await rooms.create(item.id, 0).catch((e) => {
        setStartFailed(true);
        throw e;
      });
      setInvite({ code: created.invite_code, expiresAt: created.invite_expires_at_ms });
      begin(created.room);
    },
    join: async (code) => {
      setInvite(null);
      begin(await rooms.join(code));
    },
    leave: async () => {
      const s = sessionRef.current;
      sessionRef.current = null;
      setSession(null); setRoom(null); setInvite(null);
      await s?.leave();
    },
    end: async () => {
      await sessionRef.current?.end();
    },
    renew: async () => {
      const s = sessionRef.current;
      if (!s) return;
      const r = await s.renewInvite();
      setInvite({ code: r.invite_code, expiresAt: r.invite_expires_at_ms });
    },
    openJoin: () => setJoining(true),
  };

  return (
    <Context.Provider value={value}>
      {children}
      <JoinDialog open={joining} onOpenChange={setJoining} />
      {startFailed && (
        <div role="alert" className="fixed inset-x-4 bottom-6 z-50 mx-auto flex max-w-lg items-center gap-3 rounded-2xl bg-surface-2 p-4 text-sm shadow-2xl ring-1 ring-border md:left-[264px]">
          <Users className="size-5 shrink-0 text-warning" />
          <p className="min-w-0 flex-1">{t('player:together.startFailed')}</p>
          <Button size="sm" variant="ghost" onClick={() => setStartFailed(false)}>{t('common:actions.close')}</Button>
        </div>
      )}
      {recovered && !session && (
        <RejoinBanner
          room={recovered}
          onRejoin={() => begin(recovered)}
          onLeave={() => { setRecovered(null); void rooms.leave(recovered.id).catch(() => undefined); }}
        />
      )}
    </Context.Provider>
  );
}

function RejoinBanner({ room, onRejoin, onLeave }: { room: WatchRoom; onRejoin: () => void; onLeave: () => void }) {
  const { t } = useTranslation('player');
  return (
    <div role="status" className="fixed inset-x-4 bottom-6 z-50 mx-auto flex max-w-lg flex-wrap items-center gap-3 rounded-2xl bg-surface-2 p-4 text-sm shadow-2xl ring-1 ring-border md:left-[264px]">
      <Users className="size-5 shrink-0 text-accent" />
      <p className="min-w-0 flex-1">{room.item ? t('player:together.rejoinTitle', { title: room.item.title }) : t('player:together.rejoin')}</p>
      <Button size="sm" onClick={onRejoin}>{t('player:together.rejoinAction')}</Button>
      <Button size="sm" variant="ghost" onClick={onLeave}>{t('player:together.leave')}</Button>
    </div>
  );
}

/** "Join Watch Together": type the code the host shares. */
export function JoinDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { t } = useTranslation('player');
  const { join } = useWatchTogether();
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState('');
  useEffect(() => { if (open) { setCode(''); setProblem(''); } }, [open]);
  const submit = async () => {
    setBusy(true);
    setProblem('');
    try {
      await join(code);
      onOpenChange(false);
    } catch (e) {
      const status = e instanceof RoomError ? e.status : 0;
      setProblem(status === 404 ? t('player:together.codeNotFound') : status === 429 ? t('player:together.tooManyTries') : t('player:together.joinFailed'));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(400px,calc(100vw-32px))]" title={t('player:together.joinTitle')} description={t('player:together.joinBody')}>
        <form className="flex flex-col gap-3 pb-3" onSubmit={(e) => { e.preventDefault(); void submit(); }}>
          <input
            autoFocus
            aria-label={t('player:together.code')}
            value={formatCode(code)}
            onChange={(e) => setCode(normalizeCode(e.target.value).slice(0, 8))}
            placeholder="ABCD-1234"
            autoComplete="off"
            spellCheck={false}
            className="h-12 rounded-xl bg-surface-2 px-4 text-center font-mono text-xl tracking-[0.2em] uppercase ring-1 ring-border focus:outline-none focus:ring-2 focus:ring-accent"
          />
          {problem && <p role="alert" className="text-sm text-warning">{problem}</p>}
          <Button type="submit" disabled={busy || normalizeCode(code).length !== 8}>
            {busy && <Loader2 className="animate-spin" />}{t('player:together.joinAction')}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function statusLabel(t: (k: string) => string, p: WatchParticipant) {
  if (!p.available) return t('player:together.statusUnavailable');
  if (!p.ready || p.state === 'waiting') return t('player:together.statusLoading');
  return p.state === 'playing' ? t('player:together.statusWatching') : t('player:together.statusReady');
}

/** The player's people button: who's here and their state, the invite code, Leave and End. */
export function TogetherMenu({ room, invite, onLeave, onEnd, onRenew, onOpenChange }: {
  room: WatchRoom; invite: Invite | null; onLeave: () => void; onEnd: () => void; onRenew: () => void; onOpenChange?: (open: boolean) => void;
}) {
  const { t } = useTranslation('player');
  const host = room.host_id === room.self_id;
  const [copied, setCopied] = useState(false);
  const expired = !!invite && invite.expiresAt <= Date.now();
  const copy = () => {
    if (!invite) return;
    void navigator.clipboard?.writeText(formatCode(invite.code)).then(() => { setCopied(true); window.setTimeout(() => setCopied(false), 1500); }).catch(() => undefined);
  };
  return (
    <DropdownMenu onOpenChange={onOpenChange}>
      <DropdownMenuTrigger asChild>
        <Button variant="glass" className="gap-2 px-3" aria-label={t('player:together.people', { count: room.participants.length })}>
          <Users className="size-4" />
          <span className="tabular-nums">{room.participants.length}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <DropdownMenuLabel>{t('player:together.title')}</DropdownMenuLabel>
        <ul className="flex flex-col gap-1 px-2 pb-2">
          {room.participants.map((p) => (
            <li key={p.id} className="flex items-center gap-2 text-sm">
              <span className={cn('size-2 shrink-0 rounded-full', !p.available ? 'bg-muted' : p.ready ? 'bg-accent' : 'bg-warning')} />
              <span className="min-w-0 flex-1 truncate">{p.id === room.self_id ? t('player:together.you', { name: p.name }) : p.name}</span>
              {p.id === room.host_id && <Crown className="size-3.5 text-muted" aria-label={t('player:together.host')} />}
              <span className="text-xs text-muted">{statusLabel(t, p)}</span>
            </li>
          ))}
        </ul>
        {host && (
          <>
            <DropdownMenuSeparator />
            <div className="px-2 py-2">
              <p className="text-xs text-muted">{t('player:together.inviteHint')}</p>
              {invite && !expired ? (
                <button type="button" onClick={copy} className="mt-2 flex w-full items-center justify-between rounded-lg bg-surface-2 px-3 py-2 font-mono text-lg tracking-[0.15em] ring-1 ring-border hover:ring-accent" aria-label={t('player:together.copyCode', { code: formatCode(invite.code) })}>
                  {formatCode(invite.code)}
                  {copied ? <Check className="size-4 text-accent" /> : <Copy className="size-4 text-muted" />}
                </button>
              ) : (
                <Button size="sm" variant="secondary" className="mt-2 w-full" onClick={onRenew}>{t('player:together.newCode')}</Button>
              )}
            </div>
          </>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onLeave}><LogOut className="size-4 text-muted" />{t('player:together.leave')}</DropdownMenuItem>
        {host && <DropdownMenuItem onSelect={onEnd}><X className="size-4 text-muted" />{t('player:together.endForEveryone')}</DropdownMenuItem>}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** "Waiting for Sam…" while someone's player catches up, with the seconds left before carrying on. */
export function TogetherWaiting({ room, serverNow }: { room: WatchRoom; serverNow: () => number }) {
  const { t } = useTranslation('player');
  const [, tick] = useState(0);
  useEffect(() => {
    const timer = window.setInterval(() => tick((n) => n + 1), 500);
    return () => window.clearInterval(timer);
  }, []);
  const slow = room.participants.filter((p) => p.available && !p.ready && p.id !== room.self_id).map((p) => p.name);
  const seconds = room.wait_until_ms ? Math.max(0, Math.ceil((room.wait_until_ms - serverNow()) / 1000)) : 0;
  return (
    <div role="status" className="pointer-events-none absolute inset-x-0 top-24 z-20 flex justify-center px-4">
      <div className="flex items-center gap-3 rounded-full bg-black/75 px-5 py-2.5 text-sm text-white ring-1 ring-white/15">
        <Loader2 className="size-4 animate-spin" />
        {slow.length ? t('player:together.waitingFor', { names: slow.join(', ') }) : t('player:together.waiting')}
        {seconds > 0 && <span className="tabular-nums text-white/60">{seconds}</span>}
      </div>
    </div>
  );
}

/** A short note when someone else changes the shared timeline: "Paused by Jo". */
export function useTogetherNotice(room: WatchRoom | null) {
  const { t } = useTranslation('player');
  const [notice, setNotice] = useState('');
  const last = useRef<{ revision: number; state: string; generation: number } | null>(null);
  useEffect(() => {
    if (!room) { last.current = null; return; }
    const before = last.current;
    last.current = { revision: room.revision, state: room.state, generation: room.generation };
    if (!before || room.revision <= before.revision || !room.changed_by || room.changed_by === room.self_id) return;
    const who = room.participants.find((p) => p.id === room.changed_by)?.name;
    if (!who) return;
    let text = '';
    if (room.generation !== before.generation) text = t('player:together.movedBy', { name: who });
    else if (room.state === 'paused' && before.state !== 'paused') text = t('player:together.pausedBy', { name: who });
    else if (room.state === 'playing' && before.state === 'paused') text = t('player:together.playedBy', { name: who });
    if (!text) return;
    setNotice(text);
    const timer = window.setTimeout(() => setNotice(''), 3000);
    return () => window.clearTimeout(timer);
  }, [room, t]);
  return notice;
}
