"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RefreshCw, Send } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/form";
import { EmptyState, ErrorState, InlineAlert, Skeleton } from "@/components/ui/feedback";
import { useToast } from "@/components/ui/toast";
import { messagesApi } from "@/lib/api/applications.api";
import { messageSchema } from "@/lib/validation/schemas";
import { formatDateTime } from "@/lib/utils/format";
import { cn } from "@/lib/utils/cn";
import { qk } from "@/features/case-tracker/queries";
import { useFieldError } from "@/lib/utils/use-form-error";
import type { Role } from "@/types";
import { z } from "zod";

/** Plain text only: bodies are rendered as text nodes, never as HTML. */
export function MessageThread({ applicationId, viewerRole }: { applicationId: string; viewerRole: Role }) {
  const t = useTranslations("messages");
  const fe = useFieldError();
  const locale = useLocale();
  const toast = useToast();
  const qc = useQueryClient();
  const endRef = React.useRef<HTMLDivElement>(null);

  // No push channel exists in the contract. Refresh on focus and on demand; no background polling.
  const q = useQuery({
    queryKey: qk.messages(applicationId),
    queryFn: ({ signal }) => messagesApi.list(applicationId, signal),
    refetchOnWindowFocus: true,
  });

  const form = useForm<z.infer<typeof messageSchema>>({ resolver: zodResolver(messageSchema), defaultValues: { body: "" } });
  const send = useMutation({
    mutationFn: (body: string) => messagesApi.send(applicationId, body),
    onSuccess: () => { form.reset({ body: "" }); qc.invalidateQueries({ queryKey: qk.messages(applicationId) }); },
    onError: (e) => toast({ tone: "error", title: t("sendFailed"), body: e.message }), // text stays in the box
  });

  React.useEffect(() => { endRef.current?.scrollIntoView({ block: "nearest" }); }, [q.data?.length]);

  return (
    <div className="flex h-[70dvh] min-h-96 flex-col overflow-hidden rounded-[28px] bg-white shadow-clay">
      <div className="flex items-center justify-between border-b border-line px-5 py-3">
        <p className="text-sm font-semibold text-muted">{t("secure")}</p>
        <Button variant="ghost" size="sm" onClick={() => q.refetch()} loading={q.isFetching} aria-label={t("refresh")}><RefreshCw className="h-4 w-4" aria-hidden /></Button>
      </div>
      <div className="flex-1 space-y-3 overflow-y-auto bg-canvas/60 p-4" aria-live="polite" aria-label={t("listLabel")}>
        {q.isPending && <><Skeleton className="h-14 w-2/3" /><Skeleton className="ml-auto h-14 w-1/2" /></>}
        {q.isError && <ErrorState error={q.error} onRetry={() => q.refetch()} compact />}
        {q.isSuccess && q.data.length === 0 && <EmptyState title={t("emptyTitle")} body={t("emptyBody")} />}
        {q.isSuccess && q.data.map((m) => {
          const mine = m.sender_role === viewerRole;
          return (
            <div key={m.id} className={cn("flex", mine ? "justify-end" : "justify-start")}>
              <div className={cn("max-w-[85%] rounded-[24px] border-2 border-ink p-3.5 shadow-pop", mine ? "rounded-tr-sm bg-ink text-white" : "rounded-tl-sm bg-white")}>
                <p className={cn("text-[11px] font-bold", mine ? "text-white/70" : "text-muted")}>{m.sender_name}</p>
                <p className="mt-0.5 whitespace-pre-wrap break-words text-sm font-medium">{m.body}</p>
                <p className={cn("tabular mt-1 text-[10px]", mine ? "text-white/60" : "text-faint")}>{formatDateTime(m.created_at, locale)}</p>
              </div>
            </div>
          );
        })}
        <div ref={endRef} />
      </div>
      <form onSubmit={form.handleSubmit((v) => send.mutate(v.body))} noValidate className="border-t border-line p-3">
        {send.isError && <InlineAlert tone="danger" className="mb-2">{t("sendFailedBody")}</InlineAlert>}
        <div className="flex items-end gap-2">
          <div className="flex-1">
            <label htmlFor="msg" className="sr-only">{t("compose")}</label>
            <Textarea id="msg" rows={2} placeholder={t("placeholder")} className="min-h-12" aria-invalid={!!form.formState.errors.body} {...form.register("body")} />
            {form.formState.errors.body && <p role="alert" className="mt-1 text-xs font-semibold text-danger">{fe(form.formState.errors.body)}</p>}
          </div>
          <Button type="submit" size="icon" aria-label={t("send")} loading={send.isPending}><Send className="h-4 w-4" aria-hidden /></Button>
        </div>
      </form>
    </div>
  );
}
