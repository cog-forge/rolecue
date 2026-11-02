/** What a merged recording holds, read with a strict EBML walk: every element below the Segment
 * must have a known size, as the merge writes them. */
export function webmSummary(bytes: Buffer) {
  let pos = 0;
  const vint = (keepMarker: boolean) => {
    const first = bytes[pos];
    let length = 1;
    while (length <= 8 && !(first & (0x80 >> (length - 1)))) length++;
    let value = keepMarker ? first : first & (0xff >> length);
    let ones = value === 0xff >> length;
    for (let i = 1; i < length; i++) {
      value = value * 256 + bytes[pos + i];
      ones &&= bytes[pos + i] === 0xff;
    }
    pos += length;
    return { value, unknown: !keepMarker && ones };
  };
  const summary = {
    segments: 0,
    duration: null as number | null,
    clusterTimes: [] as number[],
    blocks: 0,
    lastBlockTime: 0,
  };
  const walk = (end: number, depth: number) => {
    let clusterTime = 0;
    while (pos < end) {
      const id = vint(true).value;
      const { value, unknown } = vint(false);
      if (unknown && id !== 0x18538067)
        throw new Error(`element ${id.toString(16)} has an unknown size`);
      const bodyEnd = unknown ? end : pos + value;
      if (id === 0x18538067) {
        summary.segments++;
        walk(bodyEnd, depth + 1);
      } else if (id === 0x1549a966 || id === 0x1f43b675) {
        walk(bodyEnd, depth + 1);
      } else if (id === 0x4489) {
        summary.duration = bytes.readDoubleBE(pos);
      } else if (id === 0xe7) {
        clusterTime = 0;
        for (let i = 0; i < value; i++)
          clusterTime = clusterTime * 256 + bytes[pos + i];
        summary.clusterTimes.push(clusterTime);
      } else if (id === 0xa3) {
        summary.blocks++;
        const relative = bytes.readInt16BE(pos + 1);
        summary.lastBlockTime = Math.max(
          summary.lastBlockTime,
          clusterTime + relative,
        );
      }
      pos = bodyEnd;
    }
  };
  walk(bytes.length, 0);
  return summary;
}
