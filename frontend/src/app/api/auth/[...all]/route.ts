import { getAuth } from "@/lib/auth/server";

export const runtime = "nodejs";
export function GET(request: Request) {
  return getAuth().handler(request);
}
export function POST(request: Request) {
  return getAuth().handler(request);
}
