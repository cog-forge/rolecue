import Link from "next/link";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { reauthenticationHref } from "../utils/auth-navigation";
import {
  securityErrorCode,
  securityErrorMessage,
} from "../utils/security-errors";
import { useTranslations } from "next-intl";

export function SecurityRequestError({
  error,
  onRetry,
}: {
  error: unknown;
  onRetry?: () => void;
}) {
  const te = useTranslations("errors");
  return (
    <div role="alert" className="space-y-3">
      <p className="text-sm">{securityErrorMessage(error, te)}</p>
      {securityErrorCode(error) === "SESSION_NOT_FRESH" ? (
        <Button asChild variant="outline" className="min-h-11">
          <Link href={reauthenticationHref(routes.settings)}>
            Sign in again
          </Link>
        </Button>
      ) : (
        onRetry && (
          <Button variant="outline" className="min-h-11" onClick={onRetry}>
            Retry
          </Button>
        )
      )}
    </div>
  );
}
