"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import {
  Archive,
  ArrowLeft,
  ArrowRight,
  Check,
  Copy,
  FileSearch,
  LockKeyhole,
  Plus,
  Search,
  Trash2,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardFooter,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { readApiError } from "@/lib/i18n/api-errors";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { RubricForm } from "./rubric-form";
import { type Rubric, type RubricValues } from "../schemas/rubric-schema";
import {
  copyRubric,
  deleteRubric,
  getRubric,
  getRubricMetrics,
  listRubrics,
  rubricKeys,
  writeRubric,
} from "../api/rubrics";

export function RubricLibrary({ role }: { role: "admin" | "recruiter" }) {
  const t = useTranslations("admin.rubrics");
  const fmt = useFormatters();
  const user = useSessionUser();
  const cache = useQueryClient();
  const message = useApiErrorMessage();
  const [scope, setScope] = useState<"system" | "own">("system");
  const [status, setStatus] = useState("active");
  const [offset, setOffset] = useState(0);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [notice, setNotice] = useState(false);
  const [confirm, setConfirm] = useState<{
    kind: "archive" | "delete";
    rubric: Rubric;
  } | null>(null);
  const [reapplyToken, setReapplyToken] = useState<string | null>(null);
  const region = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      if (region.current) region.current.dataset.open = "true";
    });
    return () => cancelAnimationFrame(frame);
  }, []);
  const heading = useRef<HTMLHeadingElement>(null);
  const key = rubricKeys.all(user.id);
  const list = useQuery({
    queryKey: [...key, "list", scope, status, offset],
    queryFn: ({ signal }) => listRubrics(scope, status, offset, signal),
    enabled: selected === null,
  });
  const detail = useQuery({
    queryKey: [...key, "detail", selected],
    queryFn: ({ signal }) => getRubric(selected!, signal),
    enabled: selected !== null && selected !== "new",
    staleTime: 0,
  });
  const metrics = useQuery({
    queryKey: [...key, "metrics"],
    queryFn: ({ signal }) => getRubricMetrics(signal),
    enabled: selected !== null,
  });
  const [formRevision, setFormRevision] = useState(0);
  const save = useMutation({
    mutationFn: ({
      values,
      updatedAt,
    }: {
      values: RubricValues;
      updatedAt?: string;
    }) =>
      writeRubric({
        id: selected === "new" ? undefined : (selected ?? undefined),
        body: updatedAt
          ? { ...values, updated_at: reapplyToken ?? updatedAt }
          : values,
      }),
    onSuccess: () => {
      close();
      setScope(role === "admin" ? "system" : "own");
      setStatus("active");
      setOffset(0);
      setNotice(true);
      void cache.invalidateQueries({ queryKey: key });
    },
  });
  const action = useMutation({
    mutationFn: async ({
      kind,
      rubric,
    }: {
      kind: "copy" | "archive" | "delete";
      rubric: Rubric;
    }) => {
      if (kind === "copy") return copyRubric(rubric.id);
      if (kind === "delete") return deleteRubric(rubric.id);
      return writeRubric({
        id: rubric.id,
        body: { updated_at: rubric.updated_at, is_active: !rubric.is_active },
      });
    },
    onSuccess: (result, variables) => {
      setConfirm(null);
      setNotice(true);
      void cache.invalidateQueries({ queryKey: key });
      if (
        variables.kind === "copy" &&
        result &&
        (selected === null || selected === variables.rubric.id)
      ) {
        setScope(role === "admin" ? "system" : "own");
        cache.setQueryData([...key, "detail", result.id], result);
        setSelected(result.id);
        setReapplyToken(null);
        save.reset();
      }
    },
  });
  const recover = useMutation<Rubric, Error, "reload" | "reapply">({
    mutationFn: () => getRubric(selected!),
    onSuccess: (latest, mode) => {
      if (mode === "reload") {
        cache.setQueryData([...key, "detail", selected], latest);
        setReapplyToken(null);
        setFormRevision((revision) => revision + 1);
      } else {
        setReapplyToken(latest.updated_at);
      }
      save.reset();
    },
  });
  function close() {
    setSelected(null);
    setReapplyToken(null);
    save.reset();
    recover.reset();
  }
  function open(id: string) {
    if (action.isPending) return;
    setNotice(false);
    setSelected(id);
    setReapplyToken(null);
    save.reset();
    action.reset();
  }
  useEffect(() => {
    heading.current?.focus();
  }, [selected]);
  function retryEditor() {
    if (selected && selected !== "new") void detail.refetch();
    void metrics.refetch();
  }
  const conflict = readApiError(save.error).code === "RUBRIC_STALE";
  const current = detail.data;
  const error = list.error || action.error;
  const rows = (list.data?.items ?? []).filter((r) =>
    `${r.name} ${r.question_type} ${r.description}`
      .toLowerCase()
      .includes(search.toLowerCase()),
  );
  return (
    <div
      ref={region}
      data-open="false"
      className="t-panel-slide mx-auto w-full max-w-6xl space-y-7 pb-8"
    >
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-2">
          {!selected && (
            <p className="text-xs font-semibold tracking-widest text-rolecue-brand uppercase">
              {role === "admin" ? t("adminEyebrow") : t("recruiterEyebrow")}
            </p>
          )}
          <h1
            ref={heading}
            tabIndex={-1}
            className="text-3xl font-semibold tracking-tight outline-none"
          >
            {selected
              ? selected === "new"
                ? t("create")
                : t("rubricDetail")
              : t("title")}
          </h1>
          {!selected && (
            <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
              {role === "admin"
                ? t("adminDescription")
                : t("recruiterDescription")}
            </p>
          )}
        </div>
        {!selected && (
          <Button
            className="h-11 rounded-full bg-rolecue-brand px-5 text-white hover:bg-rolecue-brand/90 dark:text-background"
            onClick={() => open("new")}
            disabled={action.isPending}
          >
            <Plus
              aria-hidden
              className="size-4 transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover/button:rotate-90 group-focus-visible/button:rotate-90 motion-reduce:rotate-0! motion-reduce:transition-none"
            />
            {t("create")}
          </Button>
        )}
      </header>
      {notice && (
        <Alert
          role="status"
          className="border-emerald-500/25 bg-[var(--rolecue-status-on-bg)] px-4 py-3 text-[var(--rolecue-status-on-ink)]"
        >
          <Check aria-hidden />
          <AlertDescription className="text-inherit">
            {t("saved")}
          </AlertDescription>
        </Alert>
      )}
      {selected !== null ? (
        <>
          {(selected !== "new" && !detail.data && detail.isPending) ||
          (!metrics.data && metrics.isPending) ? (
            <div role="status" aria-label={t("loading")} className="space-y-4">
              <Skeleton className="h-52 rounded-2xl" />
              <Skeleton className="h-80 rounded-2xl" />
            </div>
          ) : (selected !== "new" && detail.error && !detail.data) ||
            (metrics.error && !metrics.data) ? (
            <Alert variant="destructive" className="rounded-2xl p-6">
              <AlertDescription>
                <p>{message(detail.error || metrics.error)}</p>
                <Button
                  variant="outline"
                  className="mt-4 h-11"
                  onClick={retryEditor}
                >
                  {t("retry")}
                </Button>
                <Button variant="ghost" className="h-11" onClick={close}>
                  {t("back")}
                </Button>
              </AlertDescription>
            </Alert>
          ) : (
            <>
              {current && (!current.can_manage || current.locked) && (
                <Alert className="border-rolecue-brand/25 bg-rolecue-brand/5 p-4">
                  <AlertDescription className="flex flex-wrap items-center justify-between gap-3">
                    <p className="text-sm text-muted-foreground">
                      {current.can_manage
                        ? t("references", {
                            count: current.reference_count ?? 0,
                          })
                        : t("systemReadOnly")}
                    </p>
                    <Button
                      variant="outline"
                      className="h-11"
                      disabled={action.isPending}
                      onClick={() =>
                        action.mutate({ kind: "copy", rubric: current })
                      }
                    >
                      <Copy aria-hidden />
                      {action.isPending ? t("saving") : t("copyEdit")}
                    </Button>
                  </AlertDescription>
                </Alert>
              )}
              {action.error && (
                <p role="alert" className="text-sm text-destructive">
                  {message(action.error)}
                </p>
              )}
              {(detail.error || metrics.error) && (
                <Alert variant="destructive">
                  <AlertDescription>
                    <p>{message(detail.error || metrics.error)}</p>
                    <Button
                      variant="outline"
                      className="mt-3 h-11"
                      onClick={retryEditor}
                    >
                      {t("retry")}
                    </Button>
                  </AlertDescription>
                </Alert>
              )}
              <RubricForm
                key={`${selected}-${formRevision}`}
                rubric={selected === "new" ? null : (current ?? null)}
                metrics={metrics.data ?? []}
                pending={
                  save.isPending || recover.isPending || action.isPending
                }
                error={
                  save.error || recover.error
                    ? message(save.error || recover.error)
                    : null
                }
                conflict={conflict}
                onSave={(values, updatedAt) => {
                  save.mutate({ values, updatedAt });
                }}
                onClose={close}
                onReload={() => recover.mutate("reload")}
                onReapply={() => recover.mutate("reapply")}
              />
            </>
          )}
        </>
      ) : (
        <>
          <Card className="gap-0 rounded-2xl py-0 [--card-spacing:--spacing(5)]">
            <CardHeader className="flex flex-wrap items-center justify-between gap-4 border-b bg-rolecue-brand/5 py-5 sm:flex-row">
              {role === "recruiter" ? (
                <Tabs
                  value={scope}
                  onValueChange={(value) => {
                    setScope(value === "own" ? "own" : "system");
                    setOffset(0);
                    setSearch("");
                    setStatus("active");
                  }}
                >
                  <TabsList className="h-11">
                    <TabsTrigger
                      value="system"
                      className="h-9 data-[state=active]:bg-rolecue-brand data-[state=active]:text-white dark:data-[state=active]:text-background"
                    >
                      {t("system")}
                    </TabsTrigger>
                    <TabsTrigger
                      value="own"
                      className="h-9 data-[state=active]:bg-rolecue-brand data-[state=active]:text-white dark:data-[state=active]:text-background"
                    >
                      {t("own")}
                    </TabsTrigger>
                  </TabsList>
                </Tabs>
              ) : (
                <h2 className="font-semibold">{t("system")}</h2>
              )}
              <Badge
                variant="outline"
                className="border-rolecue-brand/20 bg-rolecue-brand/10 px-3 py-1 text-rolecue-brand"
              >
                {t("count", { count: list.data?.pagination.total ?? 0 })}
              </Badge>
            </CardHeader>
            <CardContent className="px-0">
              <div className="flex flex-wrap gap-3 p-4 sm:p-5">
                <div className="relative min-w-48 flex-1">
                  <Search
                    aria-hidden
                    className="absolute top-3.5 left-3 size-4 text-muted-foreground"
                  />
                  <Input
                    aria-label={t("search")}
                    placeholder={t("search")}
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="h-11 pl-9"
                  />
                </div>
                {(role === "admin" || scope === "own") && (
                  <Select
                    value={status}
                    onValueChange={(value) => {
                      setStatus(value);
                      setOffset(0);
                    }}
                  >
                    <SelectTrigger
                      aria-label={t("status")}
                      className="h-11 min-w-40 bg-background data-[size=default]:h-11"
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent
                      position="popper"
                      className="motion-reduce:animate-none! motion-reduce:transition-none!"
                    >
                      <SelectItem value="active">{t("active")}</SelectItem>
                      <SelectItem value="archived">{t("archived")}</SelectItem>
                      <SelectItem value="all">{t("all")}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              </div>
              {error && (
                <Alert
                  variant="destructive"
                  className="mx-5 w-auto rounded-xl border-destructive/30 bg-destructive/5 p-4"
                >
                  <AlertDescription className="space-y-3">
                    <p>{message(error)}</p>
                    <Button
                      variant="outline"
                      className="h-11"
                      onClick={() => {
                        action.reset();
                        void list.refetch();
                      }}
                    >
                      {t("retry")}
                    </Button>
                  </AlertDescription>
                </Alert>
              )}
              {list.isPending ? (
                <div
                  role="status"
                  aria-label={t("loading")}
                  className="space-y-4 p-5"
                >
                  {[1, 2, 3].map((n) => (
                    <Skeleton key={n} className="h-28 rounded-xl" />
                  ))}
                </div>
              ) : !list.error && !rows.length ? (
                <div className="flex flex-col items-center gap-3 px-5 py-16 text-center">
                  <FileSearch
                    aria-hidden
                    className="size-9 text-muted-foreground"
                  />
                  <h3 className="font-semibold">
                    {search ? t("noResults") : t("empty")}
                  </h3>
                  <p className="max-w-sm text-sm text-muted-foreground">
                    {search ? t("searchHelp") : t("emptyHelp")}
                  </p>
                  <Button
                    variant="outline"
                    className="mt-2 h-11"
                    onClick={() => (search ? setSearch("") : open("new"))}
                  >
                    {!search && (
                      <Plus
                        aria-hidden
                        className="size-4 transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover/button:rotate-90 group-focus-visible/button:rotate-90 motion-reduce:rotate-0! motion-reduce:transition-none"
                      />
                    )}
                    {search ? t("clearSearch") : t("create")}
                  </Button>
                </div>
              ) : (
                <ul className="divide-y">
                  {rows.map((r) => (
                    <li
                      key={r.id}
                      className="flex flex-wrap items-center justify-between gap-4 px-5 py-5 transition-colors hover:bg-rolecue-brand/3 motion-reduce:transition-none sm:px-6"
                    >
                      <div className="min-w-0 flex-1 basis-64 space-y-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <Button
                            variant="link"
                            className="h-auto max-w-full justify-start p-0 text-left text-base font-semibold whitespace-normal text-foreground hover:text-rolecue-brand"
                            onClick={() => open(r.id)}
                            disabled={action.isPending}
                          >
                            {r.name}
                          </Button>
                          {r.locked && (
                            <Badge
                              variant="outline"
                              className="border-amber-500/30 bg-amber-500/10 text-amber-800 dark:text-amber-200"
                            >
                              <LockKeyhole
                                aria-hidden
                                className="mr-1 size-3"
                              />
                              {t("locked")}
                            </Badge>
                          )}
                          {r.is_active && !r.locked && (
                            <Badge
                              variant="outline"
                              className="border-emerald-500/25 bg-[var(--rolecue-status-on-bg)] text-[var(--rolecue-status-on-ink)]"
                            >
                              <Check aria-hidden className="mr-1 size-3" />
                              {t("active")}
                            </Badge>
                          )}
                          {!r.is_active && (
                            <Badge variant="outline">{t("archived")}</Badge>
                          )}
                        </div>
                        <p className="line-clamp-2 text-sm text-muted-foreground">
                          {r.description}
                        </p>
                        <p className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                          <span className="max-w-full rounded bg-rolecue-brand/8 px-1.5 py-0.5 text-rolecue-brand break-all">
                            {r.question_type}
                          </span>
                          <span>
                            {t("criterionCount", { count: r.criteria_count })}
                          </span>
                          <span>
                            {t("pointCount", { count: r.max_points })}
                          </span>
                          <span>{fmt.date(r.updated_at, "calendarDay")}</span>
                        </p>
                        {r.locked && (
                          <p className="text-xs text-muted-foreground">
                            {t("lockedReason", {
                              count: r.reference_count ?? 0,
                            })}
                          </p>
                        )}
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <Button
                          variant="outline"
                          className="h-11"
                          onClick={() => open(r.id)}
                          disabled={action.isPending}
                        >
                          {r.can_manage && !r.locked ? t("edit") : t("view")}
                        </Button>
                        <Button
                          variant="ghost"
                          className="h-11"
                          disabled={action.isPending}
                          onClick={() =>
                            action.mutate({ kind: "copy", rubric: r })
                          }
                        >
                          <Copy aria-hidden className="size-4" />
                          {t("copy")}
                        </Button>
                        {r.can_manage && (
                          <>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Button
                                  variant="ghost"
                                  className="size-11"
                                  disabled={action.isPending}
                                  aria-label={`${r.is_active ? t("archive") : t("restore")}: ${r.name}`}
                                  onClick={() => {
                                    action.reset();
                                    setConfirm({ kind: "archive", rubric: r });
                                  }}
                                >
                                  <Archive aria-hidden className="size-4" />
                                </Button>
                              </TooltipTrigger>
                              <TooltipContent className="motion-reduce:animate-none! motion-reduce:transition-none!">
                                {r.is_active ? t("archive") : t("restore")}
                              </TooltipContent>
                            </Tooltip>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span
                                  tabIndex={r.locked ? 0 : undefined}
                                  className="inline-flex rounded-lg focus-visible:outline-2 focus-visible:outline-ring"
                                  aria-label={
                                    r.locked
                                      ? t("lockedReason", {
                                          count: r.reference_count ?? 0,
                                        })
                                      : undefined
                                  }
                                >
                                  <Button
                                    variant="ghost"
                                    className="size-11 text-destructive hover:bg-destructive/10 hover:text-destructive focus-visible:ring-destructive/30 dark:hover:bg-destructive/20 dark:hover:text-destructive"
                                    disabled={r.locked || action.isPending}
                                    aria-label={`${t("delete")}: ${r.name}`}
                                    title={
                                      r.locked
                                        ? t("lockedReason", {
                                            count: r.reference_count ?? 0,
                                          })
                                        : t("delete")
                                    }
                                    onClick={() => {
                                      action.reset();
                                      setConfirm({ kind: "delete", rubric: r });
                                    }}
                                  >
                                    <Trash2 aria-hidden className="size-4" />
                                  </Button>
                                </span>
                              </TooltipTrigger>
                              <TooltipContent className="motion-reduce:animate-none! motion-reduce:transition-none!">
                                {r.locked
                                  ? t("lockedReason", {
                                      count: r.reference_count ?? 0,
                                    })
                                  : t("delete")}
                              </TooltipContent>
                            </Tooltip>
                          </>
                        )}
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </CardContent>
            <CardFooter className="flex flex-wrap items-center justify-between gap-3 p-4">
              <p className="text-xs text-muted-foreground">{t("pageSearch")}</p>
              <div className="flex items-center gap-3">
                <Button
                  variant="outline"
                  className="size-11"
                  aria-label={t("previous")}
                  disabled={offset === 0 || list.isFetching}
                  onClick={() => setOffset((n) => Math.max(0, n - 20))}
                >
                  <ArrowLeft aria-hidden />
                </Button>
                <span className="text-sm">{Math.floor(offset / 20) + 1}</span>
                <Button
                  variant="outline"
                  className="size-11"
                  aria-label={t("next")}
                  disabled={
                    offset + 20 >= (list.data?.pagination.total ?? 0) ||
                    list.isFetching
                  }
                  onClick={() => setOffset((n) => n + 20)}
                >
                  <ArrowRight aria-hidden />
                </Button>
              </div>
            </CardFooter>
          </Card>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {t("libraryHelp")}
          </p>
        </>
      )}
      <Dialog
        open={confirm !== null}
        onOpenChange={(open) => {
          if (!open && !action.isPending) setConfirm(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {confirm?.kind === "delete"
                ? t("deleteTitle")
                : confirm?.rubric.is_active
                  ? t("archiveTitle")
                  : t("restore")}
            </DialogTitle>
            <DialogDescription>
              {confirm?.rubric.name} —{" "}
              {confirm?.kind === "delete" ? t("deleteHelp") : t("archiveHelp")}
            </DialogDescription>
          </DialogHeader>
          {action.error && (
            <p role="alert" className="text-sm text-destructive">
              {message(action.error)}
            </p>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              className="h-11"
              disabled={action.isPending}
              onClick={() => setConfirm(null)}
            >
              {t("cancel")}
            </Button>
            <Button
              className="h-11"
              disabled={action.isPending}
              onClick={() => {
                if (confirm) action.mutate(confirm);
              }}
            >
              {action.isPending ? t("saving") : t("confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
