"use client";

import { useState } from "react";
import { LoaderCircle, Mail } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { RoleCueMark } from "@/components/brand/rolecue-mark";
import { authClient } from "@/lib/auth/client";
import { routes } from "@/config/routes";
import { SignOutButton } from "./sign-out-button";

export function VerificationNotice({
  email,
  onRetry,
  checking,
}: {
  email: string;
  onRetry: () => void;
  checking: boolean;
}) {
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState(false);

  async function send() {
    setSending(true);
    setError(false);
    try {
      const result = await authClient.sendVerificationEmail({
        email,
        callbackURL: routes.afterEmailVerification,
      });
      if (result.error) throw new Error("Verification email failed");
      setSent(true);
      gooeyToast.success("Verification email sent", {
        description: "Open the link in your inbox to continue.",
      });
    } catch {
      setError(true);
      gooeyToast.error("Email could not be sent", {
        description: "Please try again in a moment.",
      });
    } finally {
      setSending(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center px-5 py-12">
      <section
        aria-labelledby="verification-title"
        className="w-full max-w-md rounded-2xl border border-border bg-white p-6 text-center shadow-sm sm:p-8"
      >
        <RoleCueMark className="mx-auto size-24 object-contain" />
        <h1
          id="verification-title"
          className="mt-4 text-2xl font-semibold tracking-tight"
        >
          Verify your email to continue
        </h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          You signed in successfully. Your sign-in provider has not confirmed
          your email address, so RoleCue needs you to verify it before opening
          your dashboard.
        </p>
        <p className="mt-3 break-all text-sm font-semibold">{email}</p>
        {sent && (
          <p role="status" className="mt-4 text-sm text-muted-foreground">
            Check your inbox and spam folder for the verification link.
          </p>
        )}
        {error && (
          <p role="alert" className="mt-4 text-sm text-destructive">
            Unable to send the verification email. Please try again.
          </p>
        )}
        <Button
          className="mt-6 h-11 w-full rounded-xl"
          onClick={() => void send()}
          disabled={sending}
          aria-busy={sending}
        >
          {sending ? (
            <LoaderCircle className="animate-spin" aria-hidden />
          ) : (
            <Mail aria-hidden />
          )}
          {sending
            ? "Sending…"
            : sent
              ? "Resend verification email"
              : "Send verification email"}
        </Button>
        <Button
          variant="link"
          className="mt-2"
          onClick={onRetry}
          disabled={checking}
        >
          I have verified my email
        </Button>
        <div className="mt-4 border-t border-border pt-4">
          <SignOutButton />
        </div>
      </section>
    </main>
  );
}
