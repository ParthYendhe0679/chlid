import { useTranslations } from "next-intl";
import type { FieldError } from "react-hook-form";

/** Translates a zod message key (namespace "validation") from react-hook-form into display text. */
export function useFieldError() {
  const t = useTranslations("validation");
  return (err?: FieldError | { message?: string }) => {
    const m = err?.message;
    if (!m) return undefined;
    return t.has(m) ? t(m) : t("invalid");
  };
}
