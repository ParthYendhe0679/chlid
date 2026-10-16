"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/form";
import { InlineAlert } from "@/components/ui/feedback";
import { useAuth } from "@/lib/auth/auth-provider";
import { ApiError } from "@/lib/api/errors";
import { loginSchema, type LoginForm as LoginValues } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import { roleHome } from "@/lib/permissions";

/** Only same-site relative paths are accepted as a post-login target (prevents open redirects). */
function safeNext(next: string | null): string | null {
  return next && next.startsWith("/") && !next.startsWith("//") && !next.includes("\\") ? next : null;
}

export function LoginForm() {
  const t = useTranslations("auth");
  const tc = useTranslations("common");
  const fe = useFieldError();
  const router = useRouter();
  const params = useSearchParams();
  const { login } = useAuth();
  const [error, setError] = React.useState<ApiError | Error | null>(null);
  const [showOtp, setShowOtp] = React.useState(false);

  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<LoginValues>({ resolver: zodResolver(loginSchema) });

  const onSubmit = handleSubmit(async (values) => {
    setError(null);
    try {
      const user = await login({ email: values.email, password: values.password, ...(values.otp ? { otp: values.otp } : {}) });
      router.replace(safeNext(params.get("next")) ?? roleHome[user.role]);
    } catch (e) {
      setError(e as Error);
    }
  });

  const msg = !error ? null
    : error instanceof ApiError && error.code === "UNAUTHENTICATED" ? t("errors.invalidCredentials")
    : error instanceof ApiError && error.code === "RATE_LIMITED" ? t("errors.rateLimited")
    : error instanceof ApiError && error.code === "FORBIDDEN" ? error.message || t("errors.forbidden")
    : error instanceof ApiError && error.code === "NETWORK_ERROR" ? tc("errors.network")
    : error.message;

  return (
    <div>
      <h1 className="text-3xl font-extrabold">{t("loginTitle")}</h1>
      <p className="mt-2 text-sm text-ink-soft">{t("loginSubtitle")}</p>

      {params.get("expired") && <InlineAlert tone="warning" className="mt-5" title={t("sessionExpiredTitle")}>{t("sessionExpiredBody")}</InlineAlert>}
      {msg && <InlineAlert tone="danger" className="mt-5" title={t("loginFailed")}>{msg}</InlineAlert>}

      <form onSubmit={onSubmit} noValidate className="mt-6 space-y-4">
        <Field label={t("email")} htmlFor="email" error={fe(errors.email)}>
          <Input id="email" type="email" autoComplete="username" aria-invalid={!!errors.email} {...register("email")} />
        </Field>
        <Field label={t("password")} htmlFor="password" error={fe(errors.password)}>
          <Input id="password" type="password" autoComplete="current-password" aria-invalid={!!errors.password} {...register("password")} />
        </Field>
        {showOtp ? (
          <Field label={t("otp")} hint={t("otpHint")} htmlFor="otp">
            <Input id="otp" inputMode="numeric" autoComplete="one-time-code" {...register("otp")} />
          </Field>
        ) : (
          <button type="button" className="text-sm font-semibold text-purple-deep underline" onClick={() => setShowOtp(true)}>{t("useOtp")}</button>
        )}
        <Button type="submit" size="lg" className="w-full" loading={isSubmitting}>{tc("login")}</Button>
      </form>

      <p className="mt-6 text-sm">
        {t("noAccount")}{" "}
        <Link href="/register" className="font-bold text-purple-deep underline">{tc("register")}</Link>
      </p>
      <p className="mt-2 text-xs text-muted">{t("agencyNote")}</p>
    </div>
  );
}
