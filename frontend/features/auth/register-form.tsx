"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/form";
import { InlineAlert } from "@/components/ui/feedback";
import { useAuth } from "@/lib/auth/auth-provider";
import { ApiError } from "@/lib/api/errors";
import { registerSchema, type RegisterForm as RegisterValues } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import { LANG_LABEL } from "@/lib/utils/locale";
import { roleHome } from "@/lib/permissions";
import type { Lang } from "@/types";

export function RegisterForm() {
  const t = useTranslations("auth");
  const tc = useTranslations("common");
  const fe = useFieldError();
  const router = useRouter();
  const locale = useLocale() as Lang;
  const { register: signUp } = useAuth();
  const [error, setError] = React.useState<Error | null>(null);

  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<RegisterValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: { language: locale },
  });

  const onSubmit = handleSubmit(async (v) => {
    setError(null);
    try {
      const user = await signUp({ ...v, consent: true });
      router.replace(roleHome[user.role]);
    } catch (e) {
      setError(e as Error);
    }
  });

  return (
    <div>
      <h1 className="text-3xl font-extrabold">{t("registerTitle")}</h1>
      <p className="mt-2 text-sm text-ink-soft">{t("registerSubtitle")}</p>
      {error && (
        <InlineAlert tone="danger" className="mt-5" title={t("registerFailed")}>
          {error instanceof ApiError && error.code === "NETWORK_ERROR" ? tc("errors.network") : error.message}
        </InlineAlert>
      )}
      <form onSubmit={onSubmit} noValidate className="mt-6 space-y-4">
        <Field label={t("fullName")} htmlFor="full_name" error={fe(errors.full_name)}>
          <Input id="full_name" autoComplete="name" aria-invalid={!!errors.full_name} {...register("full_name")} />
        </Field>
        <Field label={t("email")} htmlFor="email" error={fe(errors.email)}>
          <Input id="email" type="email" autoComplete="email" aria-invalid={!!errors.email} {...register("email")} />
        </Field>
        <Field label={t("phone")} htmlFor="phone" error={fe(errors.phone)}>
          <Input id="phone" type="tel" autoComplete="tel" aria-invalid={!!errors.phone} {...register("phone")} />
        </Field>
        <Field label={t("password")} hint={t("passwordHint")} htmlFor="password" error={fe(errors.password)}>
          <Input id="password" type="password" autoComplete="new-password" aria-invalid={!!errors.password} {...register("password")} />
        </Field>
        <Field label={t("language")} htmlFor="language">
          <Select id="language" {...register("language")}>
            {(Object.keys(LANG_LABEL) as Lang[]).map((l) => <option key={l} value={l}>{LANG_LABEL[l]}</option>)}
          </Select>
        </Field>

        <fieldset className="rounded-2xl bg-lavender-soft p-4 text-sm">
          <legend className="px-1 text-[13px] font-bold">{t("consentTitle")}</legend>
          <ul className="mb-3 list-disc space-y-1 pl-5 text-ink-soft">
            <li>{t("consentWhat")}</li><li>{t("consentWhy")}</li><li>{t("consentWho")}</li><li>{t("consentHowLong")}</li>
          </ul>
          <label className="flex items-start gap-2 font-semibold">
            <input type="checkbox" className="mt-1 h-4 w-4 accent-purple" aria-invalid={!!errors.consent} {...register("consent")} />
            <span>{t("consentLabel")}</span>
          </label>
          {errors.consent && <p role="alert" className="mt-1 text-xs font-semibold text-danger">{fe(errors.consent)}</p>}
        </fieldset>

        <Button type="submit" size="lg" className="w-full" loading={isSubmitting}>{t("createAccount")}</Button>
      </form>
      <p className="mt-6 text-sm">
        {t("haveAccount")}{" "}
        <Link href="/login" className="font-bold text-purple-deep underline">{tc("login")}</Link>
      </p>
      <p className="mt-2 text-xs text-muted">{t("otpNote")}</p>
    </div>
  );
}
