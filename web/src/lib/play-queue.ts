import { api, type PlayQueue, type PlayQueueEdit } from './api';

/** The most entries a server queue holds; a longer list stays in this browser only. */
export const maxQueueEntries = 1000;

/**
 * Keeps the server's copy of the player's queue in step, so this device and profile can pick the
 * queue up again after a reload. Calls run one at a time, each with the revision the last one
 * returned. When an edit is refused (another tab sharing this sign-in changed the queue, or it
 * expired), the server's queue is read back and handed to onReset for the player to follow.
 */
export class QueueMirror {
  /** The server's queue as of the last call that finished. */
  queue: PlayQueue | null = null;
  /** Whether the player's queue is being mirrored (set at once, before the server answers). */
  active = false;
  private chain: Promise<unknown> = Promise.resolve();

  constructor(
    private onSynced: (queue: PlayQueue) => void,
    private onReset: (queue: PlayQueue | null) => void,
  ) {}

  private run(step: () => Promise<PlayQueue | null | undefined>, recover = true, notify = true): Promise<PlayQueue | null> {
    const done = this.chain.then(step).then(
      (queue) => {
        if (queue === undefined) return this.queue;
        this.queue = queue;
        if (queue && this.active && notify) this.onSynced(queue);
        return queue;
      },
      async () => {
        const id = this.queue?.id;
        const fresh = recover && id ? await api.playQueue(id).catch(() => null) : null;
        this.queue = fresh;
        if (!fresh) this.active = false;
        if (recover) this.onReset(fresh);
        return null;
      },
    );
    this.chain = done;
    return done;
  }

  /**
   * Starts mirroring a new queue of these titles, replacing this device's previous one. The new
   * queue (or null, when the server couldn't make it) comes back for the player to take its entries.
   */
  start(itemIds: string[]) {
    this.active = true;
    return this.run(async () => {
      const replace = this.queue ? { id: this.queue.id, revision: this.queue.revision } : undefined;
      try {
        return await api.createPlayQueue(itemIds, replace);
      } catch {
        // The queue this browser knew about changed or expired, or one is left from before a
        // reload: replace whatever the server holds now.
        const current = await api.currentPlayQueue().catch(() => null);
        return api.createPlayQueue(itemIds, current ? { id: current.id, revision: current.revision } : undefined);
      }
    }, false, false);
  }

  /** Carries on mirroring a queue read back from the server. */
  adopt(queue: PlayQueue) {
    this.active = true;
    this.run(async () => queue, false);
  }

  edit(edit: PlayQueueEdit) {
    if (!this.active) return;
    this.run(async () => (this.queue ? api.editPlayQueue(this.queue.id, this.queue.revision, edit) : undefined));
  }

  /** Points the server's queue at an entry: Advance when it is simply the next one, else Select. */
  moveTo(entryId: string) {
    if (!this.active) return;
    this.run(async () => {
      const q = this.queue;
      if (!q || q.current_entry_id === entryId) return undefined;
      const at = q.entries.findIndex((e) => e.id === q.current_entry_id);
      const edit: PlayQueueEdit = at >= 0 && q.entries[at + 1]?.id === entryId ? { operation: 'advance' } : { operation: 'select', entry_id: entryId };
      return api.editPlayQueue(q.id, q.revision, edit);
    });
  }

  /** Sets the upcoming order; entries the player doesn't know yet keep their place at the end. */
  reorder(entryIds: string[]) {
    if (!this.active) return;
    this.run(async () => {
      const q = this.queue;
      if (!q) return undefined;
      const known = new Set(q.entries.map((e) => e.id));
      const order = entryIds.filter((id) => known.has(id));
      const listed = new Set(order);
      return api.editPlayQueue(q.id, q.revision, { operation: 'reorder', entry_ids: [...order, ...q.entries.filter((e) => !listed.has(e.id)).map((e) => e.id)] });
    });
  }

  /** Playback was stopped on purpose: the server's queue is discarded. */
  end() {
    if (!this.active && !this.queue) return;
    this.active = false;
    this.run(async () => {
      if (this.queue) await api.deletePlayQueue(this.queue.id, this.queue.revision).catch(() => undefined);
      return null;
    }, false);
  }

  /** Stops mirroring but keeps the server's queue (leaving a profile), to be picked up later. */
  release() {
    this.active = false;
    this.run(async () => null, false);
  }
}
