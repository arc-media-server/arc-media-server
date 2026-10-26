/** Single-rendition adaptation: delivery/buffer evidence, not a claimed network speed test. */
export type AdaptiveObservation = {
  now: number; active: boolean; bufferSeconds: number; waiting: boolean;
  remainingSeconds: number; playbackRate: number; currentLimit: number;
  originalKbps: number; ceilingKbps: number;
};

export class AdaptiveQuality {
  private started = 0;
  private lowSince: number | null = null;
  private highSince: number | null = null;
  private changed = -Infinity;
  private switches: number[] = [];
  private sampled = -Infinity;
  private samples = 0;
  deliveryKbps = 0;
  mediaKbps = 0;
  readonly rates: number[];

  constructor(rates: number[]) { this.rates = [...rates].sort((a, b) => b - a); }
  reset(now: number) {
    this.started = now;
    this.lowSince = this.highSince = null;
    this.sampled = -Infinity; this.samples = 0;
    this.deliveryKbps = this.mediaKbps = 0;
  }
  observe(now: number, bytes: number, milliseconds: number, mediaSeconds: number) {
    if (!Number.isFinite(bytes + milliseconds + mediaSeconds) || bytes < 16384 || milliseconds < 1 || mediaSeconds <= 0) return;
    const delivery = bytes * 8 / milliseconds;
    const media = bytes * 8 / mediaSeconds / 1000;
    this.deliveryKbps = this.samples ? this.deliveryKbps * 0.7 + delivery * 0.3 : delivery;
    this.mediaKbps = this.samples ? this.mediaKbps * 0.7 + media * 0.3 : media;
    this.samples++; this.sampled = now;
  }
  decide(o: AdaptiveObservation): number | null {
    if (!o.active || o.remainingSeconds < 10 || !Number.isFinite(o.bufferSeconds) || o.playbackRate <= 0) {
      this.started = o.now; this.lowSince = this.highSince = null; return null;
    }
    if (o.now - this.started < 12000) return null;
    const original = Math.max(1, o.originalKbps || 8000);
    const ceiling = o.ceilingKbps > 0 ? o.ceilingKbps : Infinity;
    const candidates = [{ limit: 0, rate: Math.min(original, ceiling) }, ...this.rates
      .filter(q => q < original && q < ceiling).map(q => ({ limit: q, rate: q }))];
    // Keep a usable floor even when the server ceiling is below the ordinary ladder.
    if (ceiling < 720 && original > ceiling) candidates.push({ limit: ceiling, rate: ceiling });
    const fresh = o.now - this.sampled <= 20000;
    const configured = Math.min(o.currentLimit || original, ceiling);
    const current = fresh && this.mediaKbps ? this.mediaKbps : configured;
    const pressure = o.bufferSeconds < 2 && (o.waiting || fresh && this.deliveryKbps < current * o.playbackRate * 0.85);
    this.lowSince = pressure ? this.lowSince ?? o.now : null;
    this.switches = this.switches.filter(t => o.now - t < 120000);
    if (this.switches.length >= 4) return null;
    let target: number | null = null;
    if (this.lowSince !== null && o.now - this.lowSince >= 6000 && o.now - this.changed >= 20000) {
      const lower = candidates.filter(c => c.rate < Math.min(current, configured) * 0.85 && c.limit !== o.currentLimit);
      const safe = fresh ? this.deliveryKbps / o.playbackRate * 0.65 : 0;
      target = (fresh ? lower.find(c => c.rate <= safe) ?? lower.at(-1) : lower[0])?.limit ?? null;
    }
    const higher = [...candidates].reverse().find(c => c.rate > configured * 1.2 && c.limit !== o.currentLimit);
    const healthy = !!higher && o.bufferSeconds >= 25 && fresh && this.samples >= 2 && this.deliveryKbps >= higher.rate * o.playbackRate * 1.8;
    this.highSince = healthy ? this.highSince ?? o.now : null;
    if (target === null && this.highSince !== null && o.now - this.highSince >= 45000 && o.now - this.changed >= 60000) target = higher!.limit;
    if (target !== null) {
      this.changed = o.now; this.switches.push(o.now); this.lowSince = this.highSince = null;
    }
    return target;
  }
}
