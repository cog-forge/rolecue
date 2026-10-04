import type { Metadata } from "next";
import { LoginView } from "@/features/auth/components/login-view";
import { RegisterForm } from "@/features/auth/components/register-form";

export const metadata: Metadata = {
  title: "Create account — RoleCue",
  description: "Create a RoleCue account to practice technical interviews.",
};

export default function RegisterPage() {
  return (
    <LoginView>
      <RegisterForm />
    </LoginView>
  );
}
