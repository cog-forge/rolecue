// Joins the WebM files of one interview into one playable WebM.
//
// A reload starts a new MediaRecorder, and every MediaRecorder writes a whole file of its own (EBML
// header, Segment, Tracks, Clusters, with no Duration). Concatenated files do not play past the first
// one, so the clusters of the later files are moved onto the timeline of the first: each cluster gets
// its timecode shifted and an exact size, the blocks inside are copied untouched (block timecodes are
// relative to their cluster), and the result gets a Duration so players can show and seek it.
//
// MediaRecorder numbers its tracks in the order their first frames arrive, so a later file may call the
// video track 1 where the first called it 2. Tracks are matched by type and codec, and the track
// number in each block of a later file is rewritten to the first file's.
//
// Only headers are read: the reader keeps a 1 MiB window of the file, and the output refers to slices
// of the input blobs instead of copying them.

const ID = {
  ebml: 0x1a45dfa3,
  segment: 0x18538067,
  info: 0x1549a966,
  tracks: 0x1654ae6b,
  cluster: 0x1f43b675,
  timecode: 0xe7,
  simpleBlock: 0xa3,
  blockGroup: 0xa0,
  block: 0xa1,
  duration: 0x4489,
  timecodeScale: 0x2ad7b1,
  trackEntry: 0xae,
  trackNumber: 0xd7,
  trackType: 0x83,
  codecId: 0x86,
} as const;
// Everything that may sit inside a Cluster. An unknown-size cluster ends at the first other element.
const clusterChildren = new Set<number>([
  ID.timecode,
  ID.simpleBlock,
  ID.blockGroup,
  0xa7, // Position
  0xab, // PrevSize
  0x5854, // SilentTracks
  0xaf, // EncryptedBlock
]);
const WINDOW_BYTES = 1 << 20;
const MAX_HEADER_BYTES = 12;

type Range = readonly [start: number, end: number];
interface ElementHeader {
  id: number;
  /** null: unknown size (live MediaRecorder output). */
  size: number | null;
  length: number;
}
interface BlockTrack {
  /** Where the track number of the block starts, and its length in bytes. */
  pos: number;
  length: number;
  track: number;
}
interface ParsedCluster {
  timecode: number;
  body: Range[];
  blocks: BlockTrack[];
}
interface ParsedFile {
  blob: Blob;
  header: Range;
  info: Range[];
  tracks: Range;
  /** Track type and codec of each track number. */
  trackKeys: Map<number, string>;
  trackSignature: string;
  timecodeScale: number;
  clusters: ParsedCluster[];
  start: number;
  end: number;
}

class BlobWindow {
  private start = 0;
  private bytes = new Uint8Array(0);
  constructor(readonly blob: Blob) {}
  get size() {
    return this.blob.size;
  }
  /** Loads at least `length` bytes from `pos`, or fewer at the end of the file. */
  async load(pos: number, length: number) {
    const end = Math.min(this.blob.size, pos + length);
    if (pos >= this.start && end <= this.start + this.bytes.length) return;
    const until = Math.min(
      this.blob.size,
      pos + Math.max(length, WINDOW_BYTES),
    );
    this.bytes = new Uint8Array(
      await this.blob.slice(pos, until).arrayBuffer(),
    );
    this.start = pos;
  }
  /** A byte already loaded, or -1 past the end of the file. */
  at(pos: number) {
    const index = pos - this.start;
    return index >= 0 && index < this.bytes.length ? this.bytes[index] : -1;
  }
}

function vintLength(first: number) {
  for (let length = 1; length <= 8; length++)
    if (first & (0x80 >> (length - 1))) return length;
  return 0;
}

async function readHeader(
  file: BlobWindow,
  pos: number,
): Promise<ElementHeader | null> {
  await file.load(pos, MAX_HEADER_BYTES);
  const first = file.at(pos);
  const idLength = first < 0 ? 0 : vintLength(first);
  if (idLength === 0 || idLength > 4) return null;
  let id = 0;
  for (let index = 0; index < idLength; index++) {
    const byte = file.at(pos + index);
    if (byte < 0) return null;
    id = id * 256 + byte;
  }
  const sizeFirst = file.at(pos + idLength);
  const sizeLength = sizeFirst < 0 ? 0 : vintLength(sizeFirst);
  if (sizeLength === 0) return null;
  let size = sizeFirst & (0xff >> sizeLength);
  let unknown = size === 0xff >> sizeLength;
  for (let index = 1; index < sizeLength; index++) {
    const byte = file.at(pos + idLength + index);
    if (byte < 0) return null;
    unknown &&= byte === 0xff;
    size = size * 256 + byte;
  }
  return { id, size: unknown ? null : size, length: idLength + sizeLength };
}

async function readUint(file: BlobWindow, pos: number, length: number) {
  await file.load(pos, length);
  let value = 0;
  for (let index = 0; index < length; index++) {
    const byte = file.at(pos + index);
    if (byte < 0) return null;
    value = value * 256 + byte;
  }
  return value;
}

async function readText(file: BlobWindow, pos: number, length: number) {
  const bytes = new Uint8Array(
    await file.blob.slice(pos, pos + length).arrayBuffer(),
  );
  return String.fromCharCode(...bytes);
}

/** Track number and relative timecode of a SimpleBlock or Block body. */
async function readBlockTime(file: BlobWindow, pos: number) {
  await file.load(pos, MAX_HEADER_BYTES);
  const first = file.at(pos);
  const length = first < 0 ? 0 : vintLength(first);
  if (length === 0) return null;
  let track = first & (0xff >> length);
  for (let index = 1; index < length; index++) {
    const byte = file.at(pos + index);
    if (byte < 0) return null;
    track = track * 256 + byte;
  }
  const high = file.at(pos + length);
  const low = file.at(pos + length + 1);
  if (high < 0 || low < 0) return null;
  const relative = (((high << 8) | low) << 16) >> 16;
  return { track, relative, pos, length };
}

/** Child elements of a known-size master, in order. Stops at a damaged child. */
async function children(file: BlobWindow, start: number, end: number) {
  const found: (ElementHeader & { pos: number })[] = [];
  let pos = start;
  while (pos < end) {
    const header = await readHeader(file, pos);
    if (!header || header.size === null) break;
    if (pos + header.length + header.size > end) break;
    found.push({ ...header, pos });
    pos += header.length + header.size;
  }
  return found;
}

async function readTracks(file: BlobWindow, start: number, end: number) {
  const keys = new Map<number, string>();
  for (const entry of await children(file, start, end)) {
    if (entry.id !== ID.trackEntry || entry.size === null) continue;
    const body = entry.pos + entry.length;
    let number = 0;
    let type = 0;
    let codec = "";
    for (const field of await children(file, body, body + entry.size)) {
      const at = field.pos + field.length;
      const size = field.size ?? 0;
      if (field.id === ID.trackNumber)
        number = (await readUint(file, at, size)) ?? 0;
      else if (field.id === ID.trackType)
        type = (await readUint(file, at, size)) ?? 0;
      else if (field.id === ID.codecId) codec = await readText(file, at, size);
    }
    keys.set(number, `${type}:${codec}`);
  }
  const signature = [...keys.values()].sort();
  // Two tracks of the same type and codec cannot be told apart: they must keep their numbers.
  if (new Set(signature).size !== signature.length)
    for (const [number, key] of keys) keys.set(number, `${number}:${key}`);
  return { keys, signature: [...keys.values()].sort().join("|") };
}

/** Reads one MediaRecorder WebM file. A file cut short keeps every element that is whole. */
async function parseFile(blob: Blob): Promise<ParsedFile | null> {
  const file = new BlobWindow(blob);
  const ebml = await readHeader(file, 0);
  if (!ebml || ebml.id !== ID.ebml || ebml.size === null) return null;
  const headerEnd = ebml.length + ebml.size;
  const segment = await readHeader(file, headerEnd);
  if (!segment || segment.id !== ID.segment) return null;
  const segmentStart = headerEnd + segment.length;
  const segmentEnd =
    segment.size === null
      ? file.size
      : Math.min(file.size, segmentStart + segment.size);

  let info: Range[] | null = null;
  let tracks: Range | null = null;
  let trackKeys = new Map<number, string>();
  let trackSignature = "";
  let timecodeScale = 1_000_000;
  const clusters: ParsedCluster[] = [];
  // Per track: the last block time and the step before it, to place the end of the file.
  const lastTimes = new Map<number, { at: number; step: number }>();
  let first: number | null = null;

  let pos = segmentStart;
  while (pos < segmentEnd) {
    const header = await readHeader(file, pos);
    if (!header) break;
    const body = pos + header.length;
    if (header.id === ID.cluster) {
      const limit =
        header.size === null
          ? segmentEnd
          : Math.min(segmentEnd, body + header.size);
      let timecode: number | null = null;
      const ranges: Range[] = [];
      const blocks: (BlockTrack & { relative: number })[] = [];
      let at = body;
      while (at < limit) {
        const child = await readHeader(file, at);
        if (!child) break;
        if (header.size === null && !clusterChildren.has(child.id)) break;
        if (child.size === null) break;
        const end = at + child.length + child.size;
        if (end > limit) break;
        const childBody = at + child.length;
        if (child.id === ID.timecode)
          timecode = await readUint(file, childBody, child.size);
        else {
          const previous = ranges.at(-1);
          if (previous && previous[1] === at)
            ranges[ranges.length - 1] = [previous[0], end];
          else ranges.push([at, end]);
          let blockBody: number | null = null;
          if (child.id === ID.simpleBlock) blockBody = childBody;
          else if (child.id === ID.blockGroup) {
            const inner = (await children(file, childBody, end)).find(
              (item) => item.id === ID.block,
            );
            if (inner) blockBody = inner.pos + inner.length;
          }
          const time =
            blockBody === null ? null : await readBlockTime(file, blockBody);
          if (time) blocks.push(time);
        }
        at = end;
      }
      if (timecode !== null && ranges.length > 0) {
        clusters.push({
          timecode,
          body: ranges,
          blocks: blocks.map(({ pos, length, track }) => ({
            pos,
            length,
            track,
          })),
        });
        for (const block of blocks) {
          const absolute = timecode + block.relative;
          first = first === null ? absolute : Math.min(first, absolute);
          const last = lastTimes.get(block.track);
          lastTimes.set(block.track, {
            at: Math.max(absolute, last?.at ?? absolute),
            step: last ? Math.max(0, absolute - last.at) : 0,
          });
        }
      }
      // An unknown-size cluster ends where the next element starts.
      pos = header.size === null ? at : body + header.size;
      continue;
    }
    if (header.size === null) break;
    const end = body + header.size;
    if (end > segmentEnd) break;
    if (header.id === ID.info) {
      info = [];
      for (const child of await children(file, body, end)) {
        if (child.id === ID.duration) continue;
        if (child.id === ID.timecodeScale)
          timecodeScale =
            (await readUint(file, child.pos + child.length, child.size ?? 0)) ??
            timecodeScale;
        info.push([child.pos, child.pos + child.length + (child.size ?? 0)]);
      }
    } else if (header.id === ID.tracks) {
      tracks = [pos, end];
      ({ keys: trackKeys, signature: trackSignature } = await readTracks(
        file,
        body,
        end,
      ));
    }
    // SeekHead, Cues, Tags and Void point into or describe the old layout; they are dropped.
    pos = end;
  }
  if (!info || !tracks || !trackSignature) return null;
  let end = first ?? 0;
  for (const { at, step } of lastTimes.values()) end = Math.max(end, at + step);
  return {
    blob,
    header: [0, headerEnd],
    info,
    tracks,
    trackKeys,
    trackSignature,
    timecodeScale,
    clusters,
    start: first ?? 0,
    end,
  };
}

function idBytes(id: number) {
  const bytes: number[] = [];
  for (let value = id; value > 0; value = Math.floor(value / 256))
    bytes.unshift(value & 0xff);
  return bytes;
}
/** An 8-byte size, so headers never need a second pass. */
function sizeBytes(size: number) {
  const bytes = [0x01];
  for (let shift = 6; shift >= 0; shift--)
    bytes.push(Math.floor(size / 2 ** (shift * 8)) & 0xff);
  return bytes;
}
function uintElement(id: number, value: number) {
  const bytes = [...idBytes(id), 0x88];
  for (let shift = 7; shift >= 0; shift--)
    bytes.push(Math.floor(value / 2 ** (shift * 8)) & 0xff);
  return bytes;
}
function floatElement(id: number, value: number) {
  const body = new Uint8Array(8);
  new DataView(body.buffer).setFloat64(0, value);
  return [...idBytes(id), 0x88, ...body];
}
/** A track number in exactly `length` bytes, as the block had it; null if it does not fit. */
function trackBytes(track: number, length: number) {
  if (track >= 2 ** (7 * length) - 1) return null;
  const bytes: number[] = [];
  for (let index = length - 1; index >= 0; index--)
    bytes.push(Math.floor(track / 2 ** (index * 8)) & 0xff);
  bytes[0] |= 0x80 >> (length - 1);
  return new Uint8Array(bytes);
}
function rangeLength(ranges: readonly Range[]) {
  return ranges.reduce((total, [start, end]) => total + end - start, 0);
}

export interface MergedWebm {
  blob: Blob;
  /** In TimecodeScale units, milliseconds for every MediaRecorder. */
  duration: number;
}

/**
 * One WebM from the files of one recording, in order. null when a file is not WebM or the files do
 * not share the same tracks, so the caller can keep the files as they are.
 */
export async function mergeWebm(
  blobs: readonly Blob[],
): Promise<MergedWebm | null> {
  const parsed: ParsedFile[] = [];
  for (const blob of blobs) {
    const file = await parseFile(blob);
    if (!file) return null;
    if (file.clusters.length > 0) parsed.push(file);
  }
  const base = parsed[0];
  if (!base) return null;
  if (
    parsed.some(
      (file) =>
        file.trackSignature !== base.trackSignature ||
        file.timecodeScale !== base.timecodeScale,
    )
  )
    return null;

  // For each file, its track numbers as the first file numbers them.
  const baseNumbers = new Map(
    [...base.trackKeys].map(([number, key]) => [key, number]),
  );
  const renumbering = parsed.map((file) => {
    const map = new Map<number, number>();
    for (const [number, key] of file.trackKeys) {
      const target = baseNumbers.get(key);
      if (target === undefined) return null;
      if (target !== number) map.set(number, target);
    }
    return map;
  });

  const parts: BlobPart[] = [];
  const slice = (file: ParsedFile, [start, end]: Range) =>
    parts.push(file.blob.slice(start, end));
  /** The cluster body, with the track number of each block rewritten where the map says so. */
  const body = (
    file: ParsedFile,
    cluster: ParsedCluster,
    map: Map<number, number>,
  ) => {
    let blocks = 0;
    for (const [start, end] of cluster.body) {
      let cursor = start;
      for (; blocks < cluster.blocks.length; blocks++) {
        const block = cluster.blocks[blocks];
        if (block.pos >= end) break;
        const target = map.get(block.track);
        if (target === undefined) continue;
        const bytes = trackBytes(target, block.length);
        if (!bytes) return false;
        slice(file, [cursor, block.pos]);
        parts.push(bytes);
        cursor = block.pos + block.length;
      }
      slice(file, [cursor, end]);
    }
    return true;
  };
  slice(base, base.header);
  parts.push(
    new Uint8Array([
      ...idBytes(ID.segment),
      0x01,
      0xff,
      0xff,
      0xff,
      0xff,
      0xff,
      0xff,
      0xff,
    ]),
  );
  const infoPosition = parts.length;
  parts.push(new Uint8Array(0));
  base.info.forEach((range) => slice(base, range));
  const durationPosition = parts.length;
  parts.push(new Uint8Array(0));
  slice(base, base.tracks);

  let offset = 0;
  for (const [index, file] of parsed.entries()) {
    const map = renumbering[index];
    if (!map) return null;
    for (const cluster of file.clusters) {
      const timecode = Math.max(0, cluster.timecode - file.start + offset);
      const bodyLength = rangeLength(cluster.body);
      parts.push(
        new Uint8Array([
          ...idBytes(ID.cluster),
          ...sizeBytes(10 + bodyLength),
          ...uintElement(ID.timecode, timecode),
        ]),
      );
      if (!body(file, cluster, map)) return null;
    }
    offset += Math.max(0, file.end - file.start);
  }
  const duration = floatElement(ID.duration, offset);
  parts[infoPosition] = new Uint8Array([
    ...idBytes(ID.info),
    ...sizeBytes(rangeLength(base.info) + duration.length),
  ]);
  parts[durationPosition] = new Uint8Array(duration);
  return { blob: new Blob(parts, { type: "video/webm" }), duration: offset };
}
