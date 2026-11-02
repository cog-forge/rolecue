// Builds and reads small WebM files shaped like MediaRecorder output, for the merge tests.

function idBytes(id: number) {
  const bytes: number[] = [];
  for (let value = id; value > 0; value = Math.floor(value / 256))
    bytes.unshift(value & 0xff);
  return bytes;
}
function size(length: number | null) {
  if (length === null) return [0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff];
  if (length < 0x7f) return [0x80 | length];
  return [0x40 | (length >> 8), length & 0xff];
}
function el(id: number, body: number[], unknownSize = false) {
  return [...idBytes(id), ...size(unknownSize ? null : body.length), ...body];
}
function uint(id: number, value: number) {
  const body: number[] = [];
  for (let v = value; v > 0 || body.length === 0; v = Math.floor(v / 256))
    body.unshift(v & 0xff);
  return el(id, body);
}
const text = (id: number, value: string) =>
  el(
    id,
    [...value].map((char) => char.charCodeAt(0)),
  );

export interface BlockSpec {
  track: number;
  time: number;
  /** One payload byte that identifies the block in the output. */
  tag: number;
}
export interface ClusterSpec {
  time: number;
  blocks: BlockSpec[];
}
export function webmFile({
  clusters,
  tracks = [
    { number: 1, type: 2, codec: "A_OPUS" },
    { number: 2, type: 1, codec: "V_VP8" },
  ],
  unknownSizes = true,
  extra = [],
}: {
  clusters: ClusterSpec[];
  tracks?: { number: number; type: number; codec: string }[];
  unknownSizes?: boolean;
  extra?: number[];
}) {
  const header = el(0x1a45dfa3, [...uint(0x4286, 1), ...text(0x4282, "webm")]);
  const info = el(0x1549a966, [
    ...uint(0x2ad7b1, 1_000_000),
    ...text(0x4d80, "Chrome"),
  ]);
  const trackEntries = el(
    0x1654ae6b,
    tracks.flatMap((track) =>
      el(0xae, [
        ...uint(0xd7, track.number),
        ...uint(0x83, track.type),
        ...text(0x86, track.codec),
      ]),
    ),
  );
  const body = clusters.flatMap((cluster) =>
    el(
      0x1f43b675,
      [
        ...uint(0xe7, cluster.time),
        ...cluster.blocks.flatMap((block) => {
          const relative = block.time - cluster.time;
          return el(0xa3, [
            0x80 | block.track,
            (relative >> 8) & 0xff,
            relative & 0xff,
            0x80,
            block.tag,
          ]);
        }),
      ],
      unknownSizes,
    ),
  );
  const segment = el(
    0x18538067,
    [...info, ...trackEntries, ...extra, ...body],
    unknownSizes,
  );
  return new Uint8Array([...header, ...segment]);
}

export interface ReadFile {
  segments: number;
  duration: number | null;
  /** Absolute block times and tags, in file order. */
  blocks: { track: number; time: number; tag: number }[];
  topLevel: number[];
}

/** A strict reader for known sizes plus unknown-size Segment, as the merge writes them. */
export function readWebm(bytes: Uint8Array): ReadFile {
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
  const result: ReadFile = {
    segments: 0,
    duration: null,
    blocks: [],
    topLevel: [],
  };
  const walk = (end: number, depth: number, clusterTime: number) => {
    let time = clusterTime;
    while (pos < end) {
      const id = vint(true).value;
      const { value, unknown } = vint(false);
      const bodyEnd = unknown ? end : pos + value;
      if (depth === 0) result.topLevel.push(id);
      if (id === 0x18538067) {
        result.segments++;
        walk(bodyEnd, 1, 0);
      } else if (id === 0x1549a966 || id === 0x1f43b675) {
        if (unknown) throw new Error("child master of unknown size");
        walk(bodyEnd, 2, 0);
      } else if (id === 0x4489) {
        result.duration = new DataView(
          bytes.buffer,
          bytes.byteOffset + pos,
          8,
        ).getFloat64(0);
      } else if (id === 0xe7) {
        time = 0;
        for (let i = 0; i < value; i++) time = time * 256 + bytes[pos + i];
      } else if (id === 0xa3) {
        const track = bytes[pos] & 0x7f;
        const relative = (((bytes[pos + 1] << 8) | bytes[pos + 2]) << 16) >> 16;
        result.blocks.push({
          track,
          time: time + relative,
          tag: bytes[pos + 4],
        });
      }
      pos = bodyEnd;
    }
  };
  walk(bytes.length, 0, 0);
  return result;
}
