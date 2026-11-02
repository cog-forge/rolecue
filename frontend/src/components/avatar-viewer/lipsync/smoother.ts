import type { MouthSample } from "../avatar-director";

/**
 * Per-channel attack/release follower (exponential, frame-rate independent). Rising weights follow
 * `attack` seconds, falling weights `release` seconds; 0 passes the target through.
 */
export class MouthSmoother {
  private values = new Map<string, number>();
  constructor(
    private readonly attack: number,
    private readonly release: number,
  ) {}
  step(target: MouthSample, dt: number): MouthSample {
    const out: Record<string, number> = {};
    const names = new Set([...this.values.keys(), ...Object.keys(target)]);
    for (const name of names) {
      const goal = target[name] ?? 0;
      const previous = this.values.get(name) ?? 0;
      const tau = goal > previous ? this.attack : this.release;
      const rate = tau <= 0 ? 1 : 1 - Math.exp(-Math.max(0, dt) / tau);
      const value = previous + (goal - previous) * rate;
      // Settle to exact rest so silence is still, not a slow tail of tiny weights.
      if (value < 0.005 && goal === 0) this.values.delete(name);
      else {
        this.values.set(name, value);
        out[name] = value;
      }
    }
    return out;
  }
  reset() {
    this.values.clear();
  }
}
