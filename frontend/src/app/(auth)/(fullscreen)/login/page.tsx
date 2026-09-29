import type { Metadata } from "next";
import { LoginView } from "@/features/auth/components/login-view";
import { LoginForm } from "@/features/auth/components/login-form";

export const metadata: Metadata = {
  title: "Log in — RoleCue",
  description:
    "Sign in to your RoleCue account to practice technical interviews.",
};

export default function LoginPage() {
  return (
    <LoginView>
      <LoginForm />
    </LoginView>
  );
}
