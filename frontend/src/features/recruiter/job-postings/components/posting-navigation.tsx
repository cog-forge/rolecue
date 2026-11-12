"use client";

import {
  createContext,
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { usePathname, useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { routes } from "@/config/routes";
import { getPosting, jobPostingKeys } from "../api/job-postings";
import {
  getBank,
  questionBankKeys,
} from "@/features/recruiter/question-bank/api/question-bank";
import {
  evaluationKeys,
  getMetrics,
  getWeights,
} from "@/features/recruiter/evaluation/api/evaluation";
import {
  capacityKeys,
  getCapacity,
} from "@/features/recruiter/capacity/api/capacity";

type Indicator = { left: number; width: number };
export const PostingNavigationContext = createContext<{
  pendingHref: string | null;
  prefetch: (href: string) => Promise<void>;
  navigate: (href: string) => void;
  getIndicator: () => Indicator | null;
  rememberIndicator: (position: Indicator) => void;
} | null>(null);

// This boundary survives sibling route changes. Data stays in TanStack Query;
// only navigation feedback and the underline's last position live here.
export function PostingNavigation({
  postingId,
  children,
}: {
  postingId: string;
  children: ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const user = useSessionUser();
  const cache = useQueryClient();
  const [pendingHref, setPendingHref] = useState<string | null>(null);
  const indicator = useRef<Indicator | null>(null);
  const getIndicator = useCallback(() => indicator.current, []);
  const rememberIndicator = useCallback((position: Indicator) => {
    indicator.current = position;
  }, []);
  const lastPath = useRef(pathname);
  const sequence = useRef(0);
  const transition = useRef<ViewTransition | null>(null);
  const committed = useRef<{ href: string; resolve: () => void } | null>(null);
  const overview = routes.recruiter.jobPosting(postingId);
  const questions = routes.recruiter.jobPostingQuestions(postingId);
  const evaluation = routes.recruiter.jobPostingEvaluation(postingId);

  const prefetch = useCallback(
    async (href: string) => {
      router.prefetch(href);
      const posting = () =>
        cache.prefetchQuery({
          queryKey: jobPostingKeys.detail(user.id, postingId),
          queryFn: ({ signal }) => getPosting(postingId, signal),
          retry: false,
        });
      if (href === questions) {
        await Promise.all([
          posting(),
          cache.prefetchQuery({
            queryKey: questionBankKeys.detail(user.id, postingId),
            queryFn: ({ signal }) => getBank(postingId, signal),
            retry: false,
          }),
        ]);
      } else if (href === evaluation) {
        await Promise.all([
          cache.prefetchQuery({
            queryKey: evaluationKeys.weights(user.id, postingId),
            queryFn: ({ signal }) => getWeights(postingId, signal),
            retry: false,
          }),
          cache.prefetchQuery({
            queryKey: evaluationKeys.metrics(user.id),
            queryFn: ({ signal }) => getMetrics(signal),
            retry: false,
          }),
        ]);
      } else if (href === overview) {
        await Promise.all([
          posting(),
          cache.prefetchQuery({
            queryKey: capacityKeys.detail(user.id, postingId),
            queryFn: ({ signal }) => getCapacity(postingId, signal),
            retry: false,
          }),
        ]);
      }
    },
    [cache, evaluation, overview, postingId, questions, router, user.id],
  );

  useEffect(() => {
    if (![overview, questions, evaluation].includes(pathname)) return;
    // Warm the three small sibling screens after the current screen has painted.
    const timer = window.setTimeout(() => {
      for (const href of [overview, questions, evaluation]) {
        if (href !== pathname) void prefetch(href);
      }
    }, 150);
    return () => window.clearTimeout(timer);
  }, [evaluation, overview, pathname, prefetch, questions]);

  useEffect(() => {
    if (lastPath.current === pathname) return;
    lastPath.current = pathname;
    if (committed.current?.href === pathname) {
      committed.current.resolve();
      committed.current = null;
      setPendingHref(null);
    } else {
      sequence.current++;
      committed.current?.resolve();
      committed.current = null;
      transition.current?.skipTransition();
      delete document.documentElement.dataset.postingTransition;
      setPendingHref(null);
    }
  }, [pathname]);

  useEffect(
    () => () => {
      sequence.current++;
      committed.current?.resolve();
      transition.current?.skipTransition();
      delete document.documentElement.dataset.postingTransition;
    },
    [],
  );

  const navigate = async (href: string) => {
    const current = ++sequence.current;
    committed.current?.resolve();
    committed.current = null;
    transition.current?.skipTransition();
    delete document.documentElement.dataset.postingTransition;
    if (href === pathname) {
      setPendingHref(null);
      return;
    }
    setPendingHref(href);
    await prefetch(href);
    if (sequence.current !== current) return;
    const canAnimate =
      typeof document.startViewTransition === "function" &&
      !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!canAnimate) {
      router.push(href, { scroll: false });
      // Also clear pending feedback on the non-animated path after commit.
      committed.current = { href, resolve: () => {} };
      return;
    }
    const root = document.documentElement;
    root.dataset.postingTransition = "true";
    const view = document.startViewTransition(
      () =>
        new Promise<void>((resolve) => {
          // A failed or slow route must never hold the browser's snapshot indefinitely.
          const timer = window.setTimeout(() => {
            view.skipTransition();
            if (sequence.current === current) setPendingHref(null);
            resolve();
          }, 1800);
          committed.current = {
            href,
            resolve: () => {
              window.clearTimeout(timer);
              resolve();
            },
          };
          router.push(href, { scroll: false });
        }),
    );
    transition.current = view;
    void view.finished
      .catch(() => {})
      .finally(() => {
        if (sequence.current === current) {
          delete root.dataset.postingTransition;
          transition.current = null;
        }
      });
  };

  return (
    <PostingNavigationContext
      value={{
        pendingHref,
        prefetch,
        navigate: (href) => {
          void navigate(href);
        },
        getIndicator,
        rememberIndicator,
      }}
    >
      <div data-posting-panel>{children}</div>
    </PostingNavigationContext>
  );
}
