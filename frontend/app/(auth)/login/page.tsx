import { Suspense } from "react";
import { LoginForm } from "@/features/auth/login-form";

export const metadata = { title: "Log in" };

export default function LoginPage() {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  );
}
