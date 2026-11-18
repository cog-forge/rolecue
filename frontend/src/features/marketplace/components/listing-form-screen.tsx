"use client";

import { useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslations } from "next-intl";
import { gooeyToast } from "goey-toast";
import { LoaderCircle, Store } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useUnsavedChangesGuard } from "@/lib/hooks/use-unsaved-changes-guard";
import { readApiError } from "@/lib/i18n/api-errors";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  getMyListing,
  marketplaceKeys,
  saveListing,
  sellerAction,
} from "../api/marketplace";
import { eligibleAvatarsQuery } from "../eligible-avatars-query";
import {
  createListingFormSchema,
  type ListingFormValues,
  type SellerListing,
} from "../schemas/marketplace-schema";
import {
  DiscardChangesDialog,
  ListingActionDialog,
} from "./listing-action-dialog";
import {
  ListingCopy,
  ListingPreview,
  ListingStatusBadge,
  MarketplaceBack,
  MarketplaceError,
  MarketplaceLoading,
  marketplaceRoutes,
} from "./marketplace-shared";

export function ListingFormScreen({ id }: { id?: string }) {
  const t = useTranslations("marketplace");
  const user = useSessionUser();
  const params = useSearchParams();
  const listing = useQuery({
    queryKey: marketplaceKeys.listing(user.id, id ?? "new"),
    queryFn: ({ signal }) => getMyListing(id ?? "", signal),
    enabled: !!id,
    retry: false,
  });
  const eligible = useQuery({
    ...eligibleAvatarsQuery(user.id),
    enabled: !id,
    retry: false,
    refetchOnMount: "always",
  });
  return (
    <div className="mx-auto max-w-5xl space-y-6 pb-10">
      <MarketplaceBack href={marketplaceRoutes.own} />
      {(id ? listing.isPending : eligible.isPending) ? (
        <MarketplaceLoading />
      ) : (
          id
            ? listing.isError && !listing.data
            : eligible.isError && !eligible.data
        ) ? (
        <MarketplaceError
          error={id ? listing.error : eligible.error}
          retry={() => void (id ? listing.refetch() : eligible.refetch())}
        />
      ) : !id && !eligible.data?.length ? (
        <EmptyState
          icons={[Store]}
          title={t("noEligibleTitle")}
          description={t("noEligibleDescription")}
          action={{ label: t("studio"), href: "/avatar-studio" }}
        />
      ) : (
        <ListingEditor
          key={id ?? "new"}
          listing={listing.data}
          userId={user.id}
          eligible={eligible.data ?? []}
          selectedAvatar={params.get("avatar") ?? ""}
          loadError={id ? listing.error : eligible.error}
          reload={async () => {
            const result = await listing.refetch();
            if (result.isError) throw result.error;
            return result.data;
          }}
        />
      )}
    </div>
  );
}
function ListingEditor({
  listing,
  userId,
  eligible,
  selectedAvatar,
  reload,
  loadError,
}: {
  listing?: SellerListing;
  userId: string;
  eligible: { avatar_id: string; name: string }[];
  selectedAvatar: string;
  reload: () => Promise<SellerListing | undefined>;
  loadError: unknown;
}) {
  const t = useTranslations("marketplace");
  const fmt = useFormatters();
  const message = useApiErrorMessage();
  const cache = useQueryClient();
  // An incoming refetch must not overwrite an in-progress edit or replace its original version.
  const [snapshot, setSnapshot] = useState(listing);
  const [action, setAction] = useState<"submit" | "delist" | null>(null);
  const [confirmReload, setConfirmReload] = useState(false);
  const [reloadFailed, setReloadFailed] = useState<unknown>(null);
  const [reloading, setReloading] = useState(false);
  const schema = useMemo(() => createListingFormSchema((key) => t(key)), [t]);
  const initialValues = (item?: SellerListing): ListingFormValues => ({
    // The selector is create-only; the API layer explicitly omits avatar_id on PUT.
    avatar_id: item
      ? item.asset_id
      : eligible.some((avatar) => avatar.avatar_id === selectedAvatar)
        ? selectedAvatar
        : "",
    title: item?.title ?? "",
    description: item?.description ?? "",
    default_personality: item?.default_personality ?? "",
    price_coins: item?.price_coins ?? 1,
    rights_note: item?.rights_note ?? "",
  });
  const form = useForm<ListingFormValues>({
    resolver: zodResolver(schema),
    defaultValues: initialValues(snapshot),
  });
  const guard = useUnsavedChangesGuard(form.formState.isDirty);
  const canEdit = !snapshot || snapshot.can_edit;
  const save = useMutation({
    mutationFn: (values: ListingFormValues) =>
      saveListing({ id: snapshot?.id, version: snapshot?.updated_at, values }),
    onSuccess: (result) => {
      setSnapshot(result);
      form.reset(initialValues(result));
      cache.setQueryData(marketplaceKeys.listing(userId, result.id), result);
      void cache.invalidateQueries({
        queryKey: marketplaceKeys.own(userId),
        exact: false,
      });
      void cache.invalidateQueries({
        queryKey: marketplaceKeys.eligible(userId),
      });
      gooeyToast.success(t(snapshot ? "updated" : "saved"));
      if (!snapshot)
        guard.replaceAfterSave(marketplaceRoutes.listing(result.id));
    },
    retry: false,
  });
  const stale =
    readApiError(save.error).code === "MARKETPLACE_LISTING_STALE" ||
    (!!listing && !!snapshot && listing.updated_at !== snapshot.updated_at);
  async function reloadLatest() {
    setReloading(true);
    setReloadFailed(null);
    try {
      const latest = await reload();
      if (!latest) throw new Error("Listing reload did not return data");
      setSnapshot(latest);
      form.reset(initialValues(latest));
      save.reset();
    } catch (error) {
      setReloadFailed(error);
    } finally {
      setReloading(false);
      setConfirmReload(false);
    }
  }
  const pending = save.isPending || reloading;
  const errors = form.formState.errors;
  const fields = [
    {
      name: "title" as const,
      label: t("listingTitle"),
      placeholder: t("titlePlaceholder"),
      multiline: false,
    },
    {
      name: "description" as const,
      label: t("description"),
      placeholder: t("descriptionPlaceholder"),
      multiline: true,
    },
    {
      name: "default_personality" as const,
      label: t("personality"),
      hint: t("personalityHint"),
      multiline: true,
    },
    {
      name: "rights_note" as const,
      label: t("rights"),
      hint: t("rightsHint"),
      multiline: true,
    },
  ];
  return (
    <>
      {loadError !== null && (
        <ErrorState variant="compact" description={message(loadError)} />
      )}
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-2">
          <h1 className="text-3xl font-semibold tracking-tight">
            {t(snapshot ? "editTitle" : "newTitle")}
          </h1>
          <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
            {t("formHint")}
          </p>
        </div>
        {snapshot && <ListingStatusBadge status={snapshot.status} />}
      </header>
      <div className="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_18rem]">
        <div className="min-w-0 space-y-6">
          {snapshot?.review_note && (
            <section className="space-y-2 rounded-xl border border-destructive/25 bg-destructive/5 p-5">
              <h2 className="text-sm font-semibold">{t("reviewNote")}</h2>
              <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
                {snapshot.review_note}
              </p>
              {snapshot.status === "rejected" && (
                <p className="text-xs text-muted-foreground">
                  {t("rejectedHint")}
                </p>
              )}
            </section>
          )}
          {stale && (
            <ErrorState
              variant="compact"
              title={t("staleTitle")}
              description={t("staleDescription")}
              onRetry={() =>
                form.formState.isDirty
                  ? setConfirmReload(true)
                  : void reloadLatest()
              }
              isRetrying={reloading}
              retryLabel={t("reload")}
            />
          )}
          {(reloadFailed || (save.isError && !stale)) && (
            <ErrorState
              variant="compact"
              description={message(reloadFailed || save.error)}
            />
          )}
          {canEdit ? (
            <form
              noValidate
              className="space-y-6"
              onSubmit={form.handleSubmit((values) => {
                if (!pending && !stale) save.mutate(values);
              })}
            >
              {!snapshot && (
                <div className="space-y-2">
                  <label
                    id="eligible-avatar-label"
                    className="text-sm font-medium"
                  >
                    {t("avatar")}
                  </label>
                  <Select
                    defaultValue={form.getValues("avatar_id")}
                    onValueChange={(value) =>
                      form.setValue("avatar_id", value, {
                        shouldDirty: true,
                        shouldValidate: true,
                      })
                    }
                    disabled={pending}
                  >
                    <SelectTrigger
                      aria-labelledby="eligible-avatar-label"
                      aria-invalid={!!errors.avatar_id}
                      aria-describedby={
                        errors.avatar_id ? "avatar-error" : undefined
                      }
                      className="min-h-11 w-full"
                    >
                      <SelectValue placeholder={t("selectAvatar")} />
                    </SelectTrigger>
                    <SelectContent>
                      {eligible.map((avatar) => (
                        <SelectItem
                          key={avatar.avatar_id}
                          value={avatar.avatar_id}
                        >
                          {avatar.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {errors.avatar_id && (
                    <p
                      id="avatar-error"
                      role="alert"
                      className="text-sm text-destructive"
                    >
                      {errors.avatar_id.message}
                    </p>
                  )}
                </div>
              )}
              {fields.map((field) => (
                <div key={field.name} className="space-y-2">
                  <label
                    htmlFor={`listing-${field.name}`}
                    className="text-sm font-medium"
                  >
                    {field.label}
                  </label>
                  {field.multiline ? (
                    <Textarea
                      id={`listing-${field.name}`}
                      placeholder={field.placeholder}
                      rows={field.name === "description" ? 4 : 5}
                      disabled={pending}
                      aria-invalid={!!errors[field.name]}
                      aria-describedby={`listing-${field.name}-help`}
                      {...form.register(field.name)}
                    />
                  ) : (
                    <Input
                      id={`listing-${field.name}`}
                      placeholder={field.placeholder}
                      className="min-h-11"
                      disabled={pending}
                      aria-invalid={!!errors[field.name]}
                      aria-describedby={`listing-${field.name}-help`}
                      {...form.register(field.name)}
                    />
                  )}
                  <div id={`listing-${field.name}-help`}>
                    {errors[field.name] ? (
                      <p role="alert" className="text-sm text-destructive">
                        {errors[field.name]?.message}
                      </p>
                    ) : field.hint ? (
                      <p className="text-xs leading-relaxed text-muted-foreground">
                        {field.hint}
                      </p>
                    ) : null}
                  </div>
                </div>
              ))}
              <div className="max-w-xs space-y-2">
                <label htmlFor="listing-price" className="text-sm font-medium">
                  {t("price")}
                </label>
                <Input
                  id="listing-price"
                  type="number"
                  inputMode="numeric"
                  min={1}
                  max={1_000_000}
                  step={1}
                  className="min-h-11"
                  disabled={pending}
                  aria-invalid={!!errors.price_coins}
                  aria-describedby={
                    errors.price_coins ? "listing-price-error" : undefined
                  }
                  {...form.register("price_coins", { valueAsNumber: true })}
                />
                {errors.price_coins && (
                  <p
                    id="listing-price-error"
                    role="alert"
                    className="text-sm text-destructive"
                  >
                    {errors.price_coins.message}
                  </p>
                )}
              </div>
              <div className="flex flex-wrap gap-3 border-t pt-6">
                <Button
                  type="submit"
                  className="min-h-11"
                  disabled={
                    pending || stale || (!!snapshot && !form.formState.isDirty)
                  }
                  aria-busy={save.isPending}
                >
                  {save.isPending && (
                    <LoaderCircle
                      aria-hidden
                      className="size-4 motion-safe:animate-spin"
                    />
                  )}
                  {t(
                    save.isPending
                      ? "saving"
                      : snapshot
                        ? "saveChanges"
                        : "saveDraft",
                  )}
                </Button>
                {snapshot?.can_submit && (
                  <Button
                    type="button"
                    variant="outline"
                    className="min-h-11"
                    disabled={pending || stale || form.formState.isDirty}
                    onClick={() => setAction("submit")}
                  >
                    {t("submit")}
                  </Button>
                )}
              </div>
            </form>
          ) : (
            snapshot && (
              <div className="space-y-6">
                <h2 className="break-words text-xl font-semibold">
                  {snapshot.title}
                </h2>
                <ListingCopy
                  description={snapshot.description}
                  personality={snapshot.default_personality}
                />
                <section className="space-y-2 border-t pt-6">
                  <h2 className="font-semibold">{t("rights")}</h2>
                  <p className="whitespace-pre-wrap break-words text-sm leading-7 text-muted-foreground">
                    {snapshot.rights_note}
                  </p>
                </section>
              </div>
            )
          )}
        </div>
        <aside className="space-y-5 rounded-xl border bg-card/50 p-5 lg:sticky lg:top-6">
          {snapshot && (
            <>
              <ListingPreview
                path={snapshot.preview_url}
                title={snapshot.title}
              />
              <dl className="space-y-4 text-sm">
                <div>
                  <dt className="text-muted-foreground">{t("price")}</dt>
                  <dd className="mt-1 font-semibold">
                    {fmt.coins(snapshot.price_coins)}
                  </dd>
                </div>
                <div>
                  <dt className="text-muted-foreground">{t("updatedAt")}</dt>
                  <dd className="mt-1">
                    {fmt.date(snapshot.updated_at, "localDateTime")}
                  </dd>
                </div>
              </dl>
            </>
          )}
          <p className="text-xs leading-relaxed text-muted-foreground">
            {snapshot?.status === "pending"
              ? t("pendingHint")
              : snapshot?.status === "active"
                ? t("activeHint")
                : snapshot &&
                    ["delisted", "suspended"].includes(snapshot.status)
                  ? t("terminalHint")
                  : t("coinsOnly")}
          </p>
          {snapshot?.can_delist && (
            <Button
              variant="outline"
              className="min-h-11 w-full"
              disabled={pending || stale}
              onClick={() => setAction("delist")}
            >
              {t("delist")}
            </Button>
          )}
        </aside>
      </div>
      <DiscardChangesDialog
        open={guard.leaving || confirmReload}
        onStay={() => {
          guard.stay();
          setConfirmReload(false);
        }}
        onLeave={() => {
          if (confirmReload) void reloadLatest();
          else guard.leave();
        }}
      />
      {snapshot && action && (
        <ListingActionDialog
          action={action}
          onClose={() => setAction(null)}
          onReload={() => void reloadLatest()}
          onConfirm={async () => {
            const result = await sellerAction(
              snapshot.id,
              action,
              snapshot.updated_at,
            );
            setSnapshot(result);
            form.reset(initialValues(result));
            cache.setQueryData(
              marketplaceKeys.listing(userId, result.id),
              result,
            );
            void cache.invalidateQueries({
              queryKey: marketplaceKeys.all(userId),
            });
            gooeyToast.success(
              t(action === "submit" ? "submitted" : "delisted"),
            );
          }}
        />
      )}
    </>
  );
}
