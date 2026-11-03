// Recorded slices are kept in this browser until the upload is ready, so a reload or a crash in the
// middle of a recruitment interview still uploads the whole session. Every slice is keyed by the
// signed-in user and the interview, so one account never reads another's recording, and the buffer
// of an interview is deleted once its recording is ready or the Candidate gives it up.

export interface BufferedSegment {
  /** One MediaRecorder run; a reload starts the next one. Ordered by start time. */
  segment: number;
  mimeType: string;
  chunks: Blob[];
  bytes: number;
}
export interface RecordingBuffer {
  load(): Promise<BufferedSegment[]>;
  append(
    segment: number,
    seq: number,
    mimeType: string,
    blob: Blob,
  ): Promise<void>;
  clear(): Promise<void>;
}
export interface RecordingBufferStore {
  open(ownerId: string, interviewId: string): RecordingBuffer;
}

/** Slices nobody uploaded or gave up are dropped after a week. */
export const RECORDING_BUFFER_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000;
const DB_NAME = "rolecue-interview-recording";
const STORE = "chunks";

interface StoredChunk {
  owner: string;
  interview: string;
  segment: number;
  seq: number;
  mimeType: string;
  blob: Blob;
  savedAt: number;
}

function group(chunks: readonly StoredChunk[]): BufferedSegment[] {
  const segments = new Map<number, BufferedSegment>();
  const sorted = [...chunks].sort(
    (a, b) => a.segment - b.segment || a.seq - b.seq,
  );
  for (const chunk of sorted) {
    const segment = segments.get(chunk.segment) ?? {
      segment: chunk.segment,
      mimeType: chunk.mimeType,
      chunks: [],
      bytes: 0,
    };
    segment.chunks.push(chunk.blob);
    segment.bytes += chunk.blob.size;
    segments.set(chunk.segment, segment);
  }
  return [...segments.values()];
}

/** In memory only: a reload loses it. Used in tests and when IndexedDB is unavailable. */
export function createMemoryRecordingStore(
  now: () => number = Date.now,
): RecordingBufferStore {
  const chunks: StoredChunk[] = [];
  return {
    open(owner, interview) {
      const mine = (chunk: StoredChunk) =>
        chunk.owner === owner && chunk.interview === interview;
      return {
        async load() {
          return group(chunks.filter(mine));
        },
        async append(segment, seq, mimeType, blob) {
          chunks.push({
            owner,
            interview,
            segment,
            seq,
            mimeType,
            blob,
            savedAt: now(),
          });
        },
        async clear() {
          for (let index = chunks.length - 1; index >= 0; index--)
            if (mine(chunks[index])) chunks.splice(index, 1);
        },
      };
    },
  };
}

function request<T>(operation: IDBRequest<T>) {
  return new Promise<T>((resolve, reject) => {
    operation.onsuccess = () => resolve(operation.result);
    operation.onerror = () => reject(operation.error);
  });
}
function done(transaction: IDBTransaction) {
  return new Promise<void>((resolve, reject) => {
    transaction.oncomplete = () => resolve();
    transaction.onerror = transaction.onabort = () =>
      reject(transaction.error ?? new DOMException("Aborted", "AbortError"));
  });
}

function openDatabase(factory: IDBFactory) {
  const opening = factory.open(DB_NAME, 1);
  opening.onupgradeneeded = () => {
    const store = opening.result.createObjectStore(STORE, {
      keyPath: ["owner", "interview", "segment", "seq"],
    });
    store.createIndex("savedAt", "savedAt");
  };
  return request(opening);
}

/**
 * IndexedDB in this browser. If it cannot be opened (private windows of some browsers), the slices of
 * the current page are kept in memory instead; only a reload then loses the earlier part.
 */
export function createIndexedDbRecordingStore({
  factory = globalThis.indexedDB,
  now = Date.now,
}: { factory?: IDBFactory; now?: () => number } = {}): RecordingBufferStore {
  const fallback = createMemoryRecordingStore(now);
  let database: Promise<IDBDatabase | null> | null = null;
  function db() {
    database ??= (async () => {
      if (!factory) return null;
      try {
        const opened = await openDatabase(factory);
        await purge(opened);
        return opened;
      } catch {
        return null;
      }
    })();
    return database;
  }
  async function purge(opened: IDBDatabase) {
    const transaction = opened.transaction(STORE, "readwrite");
    const stale = IDBKeyRange.upperBound(now() - RECORDING_BUFFER_MAX_AGE_MS);
    const keys = await request(
      transaction.objectStore(STORE).index("savedAt").getAllKeys(stale),
    );
    keys.forEach((key) => transaction.objectStore(STORE).delete(key));
    await done(transaction);
  }
  return {
    open(owner, interview) {
      const memory = fallback.open(owner, interview);
      const range = () =>
        IDBKeyRange.bound(
          [owner, interview, -Infinity, -Infinity],
          [owner, interview, Infinity, Infinity],
        );
      // Slices this page could not write (storage full or closed). They keep their place by seq.
      const unsaved: StoredChunk[] = [];
      return {
        async load() {
          const opened = await db();
          if (!opened) return memory.load();
          const transaction = opened.transaction(STORE, "readonly");
          const chunks = (await request(
            transaction.objectStore(STORE).getAll(range()),
          )) as StoredChunk[];
          return group([...chunks, ...unsaved]);
        },
        async append(segment, seq, mimeType, blob) {
          const opened = await db();
          if (!opened) return memory.append(segment, seq, mimeType, blob);
          const chunk: StoredChunk = {
            owner,
            interview,
            segment,
            seq,
            mimeType,
            blob,
            savedAt: now(),
          };
          try {
            const transaction = opened.transaction(STORE, "readwrite");
            transaction.objectStore(STORE).put(chunk);
            await done(transaction);
          } catch {
            unsaved.push(chunk);
          }
        },
        async clear() {
          unsaved.length = 0;
          await memory.clear();
          const opened = await db();
          if (!opened) return;
          const transaction = opened.transaction(STORE, "readwrite");
          transaction.objectStore(STORE).delete(range());
          await done(transaction);
        },
      };
    },
  };
}
