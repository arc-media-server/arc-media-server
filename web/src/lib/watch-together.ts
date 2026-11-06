/**
 * Watch Together (docs/WATCH-TOGETHER-API.md): a room shares one timeline (title, position,
 * playing or paused); everyone streams their own copy. This file has no React: the room client,
 * clock sync and drift maths, used by components/WatchTogether.tsx and the player.
 */

export interface WatchParticipant {
  id: string;
  name: string;
  available: boolean;
  ready: boolean;
  /** playing, paused or waiting (buffering). */
  state: string;
}

export interface WatchRoom {
  id: string;
  revision: number;
  /** Bumps on every seek or title change: heartbeats name the one they've loaded. */
  generation: number;
  /** paused, playing, waiting (for someone buffering) or ended. */
  state: string;
  /** In the file, at server time `at_ms`. */
  position_ms: number;
  at_ms: number;
  rate: number;
  server_time_ms: number;
  wait_until_ms?: number;
  host_id: string;
  self_id: string;
  changed_by?: string;
  /** False when this profile may not play the room's current title. */
  available: boolean;
  item_id?: string;
  item?: { id: string; title: string; kind: string };
  participants: WatchParticipant[];
}

export interface WatchInvite {
  room: WatchRoom;
  invite_code: string;
  invite_expires_at_ms: number;
}

/** A refusal from the room API; `room` is the current room when the refusal was a conflict (409). */
export class RoomError extends Error {
  constructor(message: string, readonly status: number, readonly room?: WatchRoom) {
    super(message);
  }
}

const base = '/api/watch-together';

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(base + path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    let message = res.statusText;
    let room: WatchRoom | undefined;
    try {
      const data = await res.json();
      message = data.error ?? message;
      room = data.room;
    } catch {
      // not JSON
    }
    throw new RoomError(message, res.status, room);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const rooms = {
  create: (itemId: string, startMS: number) => call<WatchInvite>('POST', '', { item_id: itemId, start_ms: Math.max(0, Math.round(startMS)) }),
  join: (code: string) => call<WatchRoom>('POST', '/join', { code: normalizeCode(code) }),
  /** This device and profile's own room, or null. */
  current: () => call<WatchRoom>('GET', '/current').catch((e) => {
    if (e instanceof RoomError && e.status === 404) return null;
    throw e;
  }),
  get: (id: string) => call<WatchRoom>('GET', `/${encodeURIComponent(id)}`),
  command: (id: string, revision: number, action: string, extra: { position_ms?: number; item_id?: string } = {}) =>
    call<WatchRoom>('POST', `/${encodeURIComponent(id)}/commands`, { revision, action, ...extra }),
  heartbeat: (id: string, generation: number, positionMS: number, state: string, ready: boolean) =>
    call<WatchRoom>('POST', `/${encodeURIComponent(id)}/heartbeat`, { generation, position_ms: Math.max(0, Math.round(positionMS)), state, ready }),
  leave: (id: string) => call<void>('POST', `/${encodeURIComponent(id)}/leave`),
  renewInvite: (id: string, revision: number) => call<WatchInvite>('POST', `/${encodeURIComponent(id)}/invite`, { revision }),
  time: (clientSentMS: number) =>
    call<{ client_sent_ms: number; server_received_ms: number; server_sent_ms: number }>('GET', `/time?client_sent_ms=${Math.round(clientSentMS)}`),
};

/** "abcd 1234" or "ABCD-1234" as Arc's code. */
export function normalizeCode(input: string): string {
  return input.toUpperCase().replace(/[^A-Z0-9]/g, '');
}

/** An eight-character code as it's read aloud: XXXX-XXXX. */
export function formatCode(code: string): string {
  const c = normalizeCode(code);
  return c.length === 8 ? `${c.slice(0, 4)}-${c.slice(4)}` : c;
}

export interface ClockSample {
  /** Client send and receive times, and the server's receive and send times (epoch ms). */
  c0: number;
  c3: number;
  s1: number;
  s2: number;
}

/**
 * How far the server's clock is ahead of this one, from the sample with the shortest round trip
 * (NTP's midpoint estimate). Null with no samples.
 */
export function clockOffset(samples: ClockSample[]): number | null {
  let best: { rtt: number; offset: number } | null = null;
  for (const s of samples) {
    const rtt = s.c3 - s.c0 - (s.s2 - s.s1);
    const offset = (s.s1 - s.c0 + (s.s2 - s.c3)) / 2;
    if (!best || rtt < best.rtt) best = { rtt, offset };
  }
  return best ? best.offset : null;
}

/** Where the room should be in the file at server time `serverNow`. */
export function expectedPosition(room: Pick<WatchRoom, 'state' | 'position_ms' | 'at_ms' | 'rate'>, serverNow: number): number {
  if (room.state !== 'playing') return room.position_ms;
  return room.position_ms + Math.max(0, serverNow - room.at_ms) * (room.rate || 1);
}

/**
 * How to bring a player back in step (the handoff's thresholds): under half a second, leave it;
 * up to three seconds, play slightly slower or faster until caught up; beyond that, jump.
 */
export type Correction = { kind: 'none' } | { kind: 'rate'; rate: number } | { kind: 'seek'; to: number };
export function correction(localMS: number, expectedMS: number): Correction {
  const drift = localMS - expectedMS;
  const size = Math.abs(drift);
  if (size < 500) return { kind: 'none' };
  if (size <= 3000) return { kind: 'rate', rate: drift > 0 ? 0.97 : 1.03 };
  return { kind: 'seek', to: expectedMS };
}

/** What the player tells the room every couple of seconds. */
export interface PlayerReport {
  positionMS: number;
  /** playing, paused, or waiting while it buffers. */
  state: 'playing' | 'paused' | 'waiting';
  ready: boolean;
}

/**
 * One device's membership of a room: the live stream (full room state on every change), clock
 * sync, heartbeats and commands. `onRoom` gets every newer room; `onGone` when the room ended or
 * this membership did (404). The player supplies its state with `setReporter`.
 */
export class RoomSession {
  room: WatchRoom;
  private offset = 0;
  private source: EventSource | undefined;
  private timers: ReturnType<typeof setInterval>[] = [];
  private retry: ReturnType<typeof setTimeout> | undefined;
  private backoff = 2_000;
  private reporter: (() => PlayerReport | null) | undefined;
  private stopped = false;

  constructor(
    room: WatchRoom,
    private readonly onRoom: (room: WatchRoom) => void,
    private readonly onGone: (why: 'ended' | 'gone') => void,
  ) {
    this.room = room;
  }

  start() {
    this.open();
    void this.syncClock();
    this.timers.push(setInterval(() => void this.syncClock(), 60_000));
    this.timers.push(setInterval(() => void this.heartbeat(), 2_000));
  }

  stop() {
    this.stopped = true;
    this.source?.close();
    this.source = undefined;
    clearTimeout(this.retry);
    for (const t of this.timers) clearInterval(t);
    this.timers = [];
  }

  /** Leaves the room (and stops). */
  async leave() {
    this.stop();
    await rooms.leave(this.room.id).catch(() => undefined);
  }

  /** The server's clock now, as well as this device can tell. */
  serverNow() {
    return Date.now() + this.offset;
  }

  expected() {
    return expectedPosition(this.room, this.serverNow());
  }

  setReporter(reporter: (() => PlayerReport | null) | undefined) {
    this.reporter = reporter;
  }

  /** Reports straight away (e.g. on starting or finishing buffering) rather than on the next beat. */
  reportNow() {
    void this.heartbeat();
  }

  play() {
    return this.command('play');
  }
  pause() {
    return this.command('pause');
  }
  seek(positionMS: number) {
    return this.command('seek', { position_ms: Math.max(0, Math.round(positionMS)) });
  }
  changeItem(itemId: string, positionMS = 0) {
    return this.command('change_item', { item_id: itemId, position_ms: Math.max(0, Math.round(positionMS)) });
  }
  end() {
    return this.command('end');
  }

  async renewInvite() {
    const invite = await rooms.renewInvite(this.room.id, this.room.revision).catch(async (e) => {
      if (e instanceof RoomError && e.status === 409 && e.room) {
        this.adopt(e.room);
        return rooms.renewInvite(this.room.id, this.room.revision);
      }
      throw e;
    });
    this.adopt(invite.room);
    return invite;
  }

  /** Sends a command with the room's revision; if the room changed meanwhile, adopts it and tries once more. */
  private async command(action: string, extra: { position_ms?: number; item_id?: string } = {}) {
    try {
      this.adopt(await rooms.command(this.room.id, this.room.revision, action, extra));
    } catch (e) {
      if (!(e instanceof RoomError)) throw e;
      if (e.status === 409 && e.room) {
        this.adopt(e.room);
        this.adopt(await rooms.command(this.room.id, this.room.revision, action, extra));
        return;
      }
      if (e.status === 404) this.gone();
      throw e;
    }
  }

  private async heartbeat() {
    if (this.stopped) return;
    const r = this.reporter?.();
    if (!r) return;
    const state = r.state === 'waiting' ? 'waiting' : r.state;
    try {
      this.adopt(await rooms.heartbeat(this.room.id, this.room.generation, r.positionMS, state, r.ready && state !== 'waiting'));
    } catch (e) {
      if (!(e instanceof RoomError)) return; // offline for a moment: the next beat tries again
      if (e.status === 409 && e.room) this.adopt(e.room); // a seek or new title: report against it next time
      else if (e.status === 404) this.gone();
    }
  }

  private async syncClock() {
    const samples: ClockSample[] = [];
    for (let i = 0; i < 3; i++) {
      const c0 = Date.now();
      try {
        const t = await rooms.time(c0);
        samples.push({ c0, c3: Date.now(), s1: t.server_received_ms, s2: t.server_sent_ms });
      } catch {
        break;
      }
    }
    const offset = clockOffset(samples);
    if (offset !== null) this.offset = offset;
  }

  /** Takes a room from the server if it's at least as new as the one held. */
  adopt(room: WatchRoom) {
    if (this.stopped) return;
    if (room.revision < this.room.revision && room.id === this.room.id) return;
    this.room = room;
    this.onRoom(room);
    if (room.state === 'ended') {
      this.stop();
      this.onGone('ended');
    }
  }

  private gone() {
    if (this.stopped) return;
    this.stop();
    this.onGone('gone');
  }

  private open() {
    if (this.stopped) return;
    const es = new EventSource(`${base}/${encodeURIComponent(this.room.id)}/events`, { withCredentials: true });
    this.source = es;
    es.addEventListener('open', () => {
      this.backoff = 2_000;
    });
    es.addEventListener('room', (e) => {
      try {
        this.adopt(JSON.parse((e as MessageEvent).data) as WatchRoom);
      } catch {
        // skip a malformed frame; the next carries the full room again
      }
    });
    es.addEventListener('error', () => {
      if (es.readyState !== EventSource.CLOSED || this.stopped) return;
      // The browser gave up (a refusal): find out whether the room is still ours, then reopen.
      es.close();
      this.source = undefined;
      rooms
        .get(this.room.id)
        .then((room) => {
          this.adopt(room);
          this.retry = setTimeout(() => this.open(), this.backoff);
          this.backoff = Math.min(this.backoff * 2, 30_000);
        })
        .catch((err) => {
          if (err instanceof RoomError && err.status === 404) this.gone();
          else {
            this.retry = setTimeout(() => this.open(), this.backoff);
            this.backoff = Math.min(this.backoff * 2, 30_000);
          }
        });
    });
  }
}
