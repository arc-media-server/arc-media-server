import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Check, Copy, Crown, Link2, Loader2, LogOut, Play, UserPlus, Users, X } from 'lucide-react';
import { api, type Item } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { formatCode, normalizeCode, RoomError, RoomSession, rooms, type ListedRoom, type WatchParticipant, type WatchPerson, type WatchRoom } from '@/lib/watch-together';
import { Avatar } from './Avatar';
import { Button } from './ui/button';
import { Dialog, DialogContent } from './ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from './ui/dropdown-menu';
import { cn } from '@/lib/utils';

interface Invite {
  code: string;
  expiresAt: number;
  /** The room it's for, when it was made before anyone joined. */
  roomId?: string;
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
  /** A room with a code, opened straight away in this player. */
  start: (item: Item) => Promise<void>;
  /** Choose who to invite to watch `item`; Done makes the room (listed on Home). */
  inviteTo: (item: Item) => void;
  /** Invite more people to the room this device is in (host). */
  inviteMore: () => void;
  /** Joins a room from Home's Watch Together row. */
  accept: (room: ListedRoom) => Promise<void>;
  join: (code: string) => Promise<void>;
  leave: () => Promise<void>;
  end: () => Promise<void>;
  renew: () => Promise<void>;
  /** Opens the dialog for typing a code. */
  openJoin: () => void;
}

const Context = createContext<WatchTogether>({
  available: false, session: null, room: null, invite: null, ended: null,
  start: async () => {}, inviteTo: () => {}, inviteMore: () => {}, accept: async () => {}, join: async () => {}, leave: async () => {}, end: async () => {}, renew: async () => {}, openJoin: () => {},
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
  const [joinCode, setJoinCode] = useState('');
  const [recovered, setRecovered] = useState<WatchRoom | null>(null);
  const [startFailed, setStartFailed] = useState(false);
  const [choosing, setChoosing] = useState<{ item: Item } | { roomId: string; itemId: string } | null>(null);
  const [made, setMade] = useState<ListedRoom | null>(null);
  const qc = useQueryClient();
  const refreshRooms = useCallback(() => void qc.invalidateQueries({ queryKey: ['watch-together-rooms'] }), [qc]);
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

  // An invite link (/?together=CODE) opens Join with the code filled in.
  useEffect(() => {
    if (!available) return;
    const url = new URL(window.location.href);
    const code = url.searchParams.get('together');
    if (!code) return;
    url.searchParams.delete('together');
    window.history.replaceState(window.history.state, '', url.pathname + url.search + url.hash);
    setJoinCode(normalizeCode(code).slice(0, 8));
    setJoining(true);
  }, [available]);

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
    inviteTo: (item) => setChoosing({ item }),
    inviteMore: () => {
      const r = sessionRef.current?.room;
      if (r?.item_id) setChoosing({ roomId: r.id, itemId: r.item_id });
    },
    accept: async (listed) => {
      const r = await rooms.accept(listed.id);
      // Keep the code from making the room, for the host's lobby.
      setInvite((old) => (old?.roomId === listed.id ? old : null));
      setMade(null);
      begin(r);
      refreshRooms();
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
    openJoin: () => { setJoinCode(''); setJoining(true); },
  };

  return (
    <Context.Provider value={value}>
      {children}
      <JoinDialog open={joining} onOpenChange={setJoining} initialCode={joinCode} />
      <InviteDialog
        target={choosing}
        onClose={() => setChoosing(null)}
        onCode={(item) => { setChoosing(null); void value.start(item).catch(() => undefined); }}
        onMade={(created) => {
          setChoosing(null);
          setInvite({ code: created.invite_code, expiresAt: created.invite_expires_at_ms, roomId: created.room.id });
          setMade(created.room);
          refreshRooms();
        }}
        onAdded={(r) => { setChoosing(null); sessionRef.current?.adopt(r); }}
      />
      {made && !session && (
        <div role="status" className="fixed inset-x-4 bottom-6 z-50 mx-auto flex max-w-lg flex-wrap items-center gap-3 rounded-2xl bg-surface-2 p-4 text-sm shadow-2xl ring-1 ring-border md:left-[264px]">
          <Users className="size-5 shrink-0 text-accent" />
          <p className="min-w-0 flex-1">{t('player:together.roomMade', { title: made.item.title })}</p>
          <Button size="sm" variant="primary" onClick={() => void value.accept(made).catch(() => setStartFailed(true))}>{t('player:together.joinNow')}</Button>
          <Button size="sm" variant="ghost" onClick={() => setMade(null)}>{t('common:actions.close')}</Button>
        </div>
      )}
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
      <Button size="sm" variant="primary" onClick={onRejoin}>{t('player:together.rejoinAction')}</Button>
      <Button size="sm" variant="ghost" onClick={onLeave}>{t('player:together.leave')}</Button>
    </div>
  );
}

/**
 * Choose who to invite. For a new room, Done makes it (nobody's in it until they join from Home's
 * Watch Together row); inside a room, Done invites more people to it.
 */
function InviteDialog({ target, onClose, onCode, onMade, onAdded }: {
  target: { item: Item } | { roomId: string; itemId: string } | null;
  onClose: () => void;
  onCode: (item: Item) => void;
  onMade: (created: { room: ListedRoom; invite_code: string; invite_expires_at_ms: number }) => void;
  onAdded: (room: WatchRoom) => void;
}) {
  const { t } = useTranslation('player');
  const itemId = target ? ('item' in target ? target.item.id : target.itemId) : undefined;
  const people = useQuery({ queryKey: ['watch-together-people', itemId], queryFn: () => rooms.people(itemId), enabled: !!target, staleTime: 30_000 });
  const [chosen, setChosen] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState('');
  useEffect(() => { if (target) { setChosen([]); setProblem(''); } }, [target]);
  const toggle = (p: WatchPerson) => setChosen((c) => (c.includes(p.id) ? c.filter((id) => id !== p.id) : [...c, p.id]));
  const done = async () => {
    if (!target || !chosen.length) return;
    setBusy(true);
    setProblem('');
    try {
      if ('item' in target) onMade(await rooms.createInvited(target.item.id, chosen));
      else onAdded(await rooms.invitePeople(target.roomId, chosen));
    } catch (e) {
      setProblem(e instanceof RoomError && e.status === 429 ? t('player:together.tooManyRooms') : t('player:together.inviteFailed'));
    } finally {
      setBusy(false);
    }
  };
  const list = people.data ?? [];
  return (
    <Dialog open={!!target} onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent
        className="w-[min(440px,calc(100vw-32px))]"
        title={t('player:together.inviteTitle')}
        description={target && 'item' in target ? t('player:together.inviteBody', { title: target.item.title }) : t('player:together.inviteMoreBody')}
        footer={
          <>
            {target && 'item' in target && (
              <Button variant="ghost" className="mr-auto" onClick={() => onCode(target.item)}>{t('player:together.useCode')}</Button>
            )}
            <Button variant="primary" disabled={busy || !chosen.length} onClick={() => void done()}>
              {busy && <Loader2 className="animate-spin" />}{t('player:together.done')}
            </Button>
          </>
        }
      >
        {people.isPending ? (
          <div className="flex justify-center py-6"><Loader2 className="size-5 animate-spin text-muted" /></div>
        ) : people.error ? (
          <p role="alert" className="py-2 text-sm text-warning">{t('player:together.peopleFailed')}</p>
        ) : !list.length ? (
          <p className="py-2 text-sm text-muted">{t('player:together.nobodyToInvite')}</p>
        ) : (
          <ul className="-mx-2 flex flex-col pb-2" aria-label={t('player:together.inviteList')}>
            {list.map((p) => {
              const on = chosen.includes(p.id);
              return (
                <li key={p.id}>
                  <button type="button" role="checkbox" aria-checked={on} disabled={!p.can_watch} onClick={() => toggle(p)}
                    className="flex w-full items-center gap-3 rounded-xl px-2 py-2 text-left hover:bg-surface-2 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent">
                    <Avatar name={p.name} color={p.color} className="size-9 text-sm" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate font-medium">{p.name}</span>
                      <span className="block text-xs text-muted">{!p.can_watch ? t('player:together.cantWatch') : p.friend ? t('player:together.friend') : t('player:together.household')}</span>
                    </span>
                    <span className={cn('grid size-5 shrink-0 place-items-center rounded-md ring-1 ring-inset', on ? 'bg-accent-strong text-accent-foreground ring-accent-strong' : 'ring-border')}>
                      {on && <Check className="size-3.5" />}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
        {problem && <p role="alert" className="pb-2 text-sm text-warning">{problem}</p>}
      </DialogContent>
    </Dialog>
  );
}

/** Home's Watch Together row: rooms this profile made, was invited to or is in. */
export function useWatchRooms() {
  const { available } = useWatchTogether();
  const profileId = useAuth().data?.profile?.id;
  return useQuery({ queryKey: ['watch-together-rooms', profileId], queryFn: rooms.list, enabled: available && !!profileId, refetchInterval: 60_000 });
}

/** One room on Home: the title's artwork, who's hosting and who's there, Join and Decline. */
export function WatchRoomCard({ room }: { room: ListedRoom }) {
  const { t } = useTranslation('player');
  const together = useWatchTogether();
  const qc = useQueryClient();
  const item = useQuery({ queryKey: ['item', room.item.id], queryFn: () => api.item(room.item.id), staleTime: 5 * 60_000 });
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState('');
  const art = item.data?.images?.backdrop ?? item.data?.images?.series_backdrop ?? item.data?.images?.primary;
  const subtitle = item.data?.kind === 'episode' && item.data.series_title ? item.data.series_title : undefined;
  const people = room.participants.length
    ? t('player:together.watchingNow', { count: room.participants.length, names: room.participants.join(', ') })
    : room.invited.length
      ? t('player:together.invitedNames', { names: room.invited.join(', ') })
      : '';
  const join = async () => {
    setBusy(true);
    setProblem('');
    try {
      await together.accept(room);
    } catch {
      setProblem(t('player:together.joinFailed'));
      void qc.invalidateQueries({ queryKey: ['watch-together-rooms'] });
    } finally {
      setBusy(false);
    }
  };
  const dismiss = async () => {
    setBusy(true);
    await rooms.dismiss(room.id).catch(() => undefined);
    setBusy(false);
    void qc.invalidateQueries({ queryKey: ['watch-together-rooms'] });
  };
  return (
    <article aria-label={room.item.title} className="flex w-[320px] shrink-0 snap-start flex-col overflow-hidden rounded-2xl bg-surface ring-1 ring-border sm:w-[360px]">
      <div className="relative aspect-video bg-surface-2">
        {art && <img src={art} alt="" className="absolute inset-0 size-full object-cover" loading="lazy" />}
        <div className="absolute inset-0 bg-gradient-to-t from-black/85 via-black/20 to-transparent" />
        <div className="absolute left-3 top-3 flex items-center gap-1.5 rounded-full bg-black/60 px-2.5 py-1 text-xs font-medium text-white backdrop-blur">
          <Users className="size-3.5" />
          {room.state === 'playing' || room.state === 'waiting' ? t('player:together.cardPlaying') : t('player:together.title')}
        </div>
        <div className="absolute inset-x-3 bottom-3 text-white">
          {subtitle && <p className="truncate text-xs text-white/70">{subtitle}</p>}
          <p className="truncate text-base font-semibold">{room.item.title}</p>
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-3 p-3">
        <div className="min-w-0 text-sm">
          <p className="truncate font-medium">{room.hosting ? t('player:together.youreHosting') : t('player:together.hostedBy', { name: room.host_name })}</p>
          {people && <p className="truncate text-xs text-muted">{people}</p>}
        </div>
        {problem && <p role="alert" className="text-xs text-warning">{problem}</p>}
        <div className="mt-auto flex gap-2">
          <Button size="sm" variant="primary" className="flex-1" disabled={busy} onClick={() => void join()}>
            {busy ? <Loader2 className="animate-spin" /> : <Play />}{room.joined ? t('player:together.return') : t('player:together.joinAction')}
          </Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={() => void dismiss()}>
            {room.hosting ? t('player:together.cancelRoom') : t('player:together.decline')}
          </Button>
        </div>
      </div>
    </article>
  );
}

/** "Join Watch Together": type the code the host shares. */
export function JoinDialog({ open, onOpenChange, initialCode = '' }: { open: boolean; onOpenChange: (open: boolean) => void; initialCode?: string }) {
  const { t } = useTranslation('player');
  const { join } = useWatchTogether();
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState('');
  useEffect(() => { if (open) { setCode(initialCode); setProblem(''); } }, [open, initialCode]);
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
          <Button type="submit" variant="primary" disabled={busy || normalizeCode(code).length !== 8}>
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

/** Who's in the room, with the host's crown and each person's state. */
function ParticipantList({ room, className }: { room: WatchRoom; className?: string }) {
  const { t } = useTranslation('player');
  return (
    <ul className={cn('flex flex-col gap-1', className)}>
      {(room.invited ?? []).map((p, i) => (
        <li key={`invited-${i}`} className="order-last flex items-center gap-2 text-sm text-muted">
          <span className="size-2 shrink-0 rounded-full ring-1 ring-inset ring-muted" />
          <span className="min-w-0 flex-1 truncate">{p.name}</span>
          <span className="text-xs">{t('player:together.statusInvited')}</span>
        </li>
      ))}
      {room.participants.map((p) => (
        <li key={p.id} className="flex items-center gap-2 text-sm">
          <span className={cn('size-2 shrink-0 rounded-full', !p.available ? 'bg-muted' : p.ready ? 'bg-accent' : 'bg-warning')} />
          <span className="min-w-0 flex-1 truncate">{p.id === room.self_id ? t('player:together.you', { name: p.name }) : p.name}</span>
          {p.id === room.host_id && <Crown className="size-3.5 text-muted" aria-label={t('player:together.host')} />}
          <span className="text-xs text-muted">{statusLabel(t, p)}</span>
        </li>
      ))}
    </ul>
  );
}

/** The link that opens Join Watch Together with the code filled in. */
export const inviteLink = (code: string) => `${window.location.origin}/?together=${formatCode(code)}`;

/** The host's invite: the code (click to copy) and the link, or "Get a new code" once it has run out. */
function InviteCode({ invite, onRenew, large }: { invite: Invite | null; onRenew: () => void; large?: boolean }) {
  const { t } = useTranslation('player');
  const [copied, setCopied] = useState<'code' | 'link' | null>(null);
  const expired = !invite || invite.expiresAt <= Date.now();
  const copy = (what: 'code' | 'link') => {
    if (!invite) return;
    const text = what === 'code' ? formatCode(invite.code) : inviteLink(invite.code);
    void navigator.clipboard?.writeText(text).then(() => { setCopied(what); window.setTimeout(() => setCopied(null), 1500); }).catch(() => undefined);
  };
  if (expired) return <Button size="sm" variant="secondary" className="w-full" onClick={onRenew}>{t('player:together.newCode')}</Button>;
  return (
    <div className="flex flex-col gap-2">
      <button type="button" onClick={() => copy('code')}
        className={cn('flex w-full items-center justify-between rounded-lg bg-surface-2 px-3 font-mono tracking-[0.15em] ring-1 ring-border hover:ring-accent', large ? 'py-3 text-2xl' : 'py-2 text-lg')}
        aria-label={t('player:together.copyCode', { code: formatCode(invite.code) })}>
        {formatCode(invite.code)}
        {copied === 'code' ? <Check className="size-4 text-accent" /> : <Copy className="size-4 text-muted" />}
      </button>
      <Button size="sm" variant="secondary" onClick={() => copy('link')}>
        {copied === 'link' ? <Check className="text-accent" /> : <Link2 />}
        {copied === 'link' ? t('player:together.linkCopied') : t('player:together.copyLink')}
      </Button>
    </div>
  );
}

/** The player's people button: who's here and their state, the invite code, Leave and End. */
export function TogetherMenu({ room, invite, onLeave, onEnd, onRenew, onOpenChange }: {
  room: WatchRoom; invite: Invite | null; onLeave: () => void; onEnd: () => void; onRenew: () => void; onOpenChange?: (open: boolean) => void;
}) {
  const { t } = useTranslation('player');
  const host = room.host_id === room.self_id;
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
        <ParticipantList room={room} className="px-2 pb-2" />
        {host && (
          <>
            <DropdownMenuSeparator />
            <div className="flex flex-col gap-2 px-2 py-2">
              <p className="text-xs text-muted">{t('player:together.inviteHint')}</p>
              <InviteCode invite={invite} onRenew={onRenew} />
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

/**
 * Before a room first plays: the host invites people and starts when they're in; the others see
 * who's here and wait for the host.
 */
export function TogetherLobby({ room, invite, onStart, onLeave, onRenew, onInvite }: {
  room: WatchRoom; invite: Invite | null; onStart: () => void; onLeave: () => void; onRenew: () => void; onInvite: () => void;
}) {
  const { t } = useTranslation('player');
  const host = room.host_id === room.self_id;
  const hostName = room.participants.find((p) => p.id === room.host_id)?.name ?? room.host_name ?? '';
  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
      <section role="dialog" aria-modal="false" aria-labelledby="together-lobby-title"
        className="glass flex w-full max-w-sm flex-col gap-4 rounded-3xl border border-border p-6 text-foreground shadow-2xl shadow-black/50">
        <div className="flex items-center gap-3">
          <Users className="size-5 shrink-0 text-accent" />
          <div className="min-w-0">
            <h2 id="together-lobby-title" className="text-lg font-semibold">{t('player:together.title')}</h2>
            {room.item && <p className="truncate text-sm text-muted">{room.item.title}</p>}
          </div>
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs font-medium uppercase tracking-wide text-subtle">{t('player:together.inRoom', { count: room.participants.length })}</p>
            {host && <Button size="sm" variant="ghost" className="-mr-2" onClick={onInvite}><UserPlus />{t('player:together.invitePeople')}</Button>}
          </div>
          <ParticipantList room={room} />
        </div>
        {host && (
          <details className="group text-sm">
            <summary className="cursor-pointer select-none text-muted hover:text-foreground">{t('player:together.shareCode')}</summary>
            <div className="mt-2 flex flex-col gap-2">
              <p className="text-xs text-muted">{t('player:together.lobbyInvite')}</p>
              <InviteCode invite={invite} onRenew={onRenew} />
            </div>
          </details>
        )}
        {host ? (
          <div className="flex gap-2">
            <Button variant="primary" className="flex-1" onClick={onStart}><Play />{t('player:together.start')}</Button>
            <Button variant="ghost" onClick={onLeave}>{t('player:together.leave')}</Button>
          </div>
        ) : (
          <div className="flex items-center gap-3">
            <Loader2 className="size-4 shrink-0 animate-spin text-muted" />
            <p className="min-w-0 flex-1 text-sm text-muted">{t('player:together.waitingForHost', { name: hostName })}</p>
            <Button variant="ghost" size="sm" onClick={onLeave}>{t('player:together.leave')}</Button>
          </div>
        )}
      </section>
    </div>
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
