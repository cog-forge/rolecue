import type { RoleProfile, UploadTicket } from "./contract";

// Public interface shared by candidate practice and recruiter-owned JD preparation.
// Each owner supplies its real API, cache keys, navigation and audience copy.
export interface JDWorkflow {
  audience: "candidate" | "recruiter";
  api: {
    create: (rawText: string) => Promise<RoleProfile>;
    get: (id: string, signal?: AbortSignal) => Promise<RoleProfile>;
    analyze: (id: string, signal?: AbortSignal) => Promise<RoleProfile>;
    createUpload: (file: File, signal: AbortSignal) => Promise<UploadTicket>;
    completeUpload: (id: string, signal: AbortSignal) => Promise<RoleProfile>;
  };
  keys: {
    detail: (userId: string, id: string) => readonly string[];
    lists: (userId: string) => readonly string[];
  };
  links: { list: string; detail: (id: string) => string };
}
