"use client";

import * as React from "react";
import { Bot, Check, ChevronDown, Copy, Mic, MicOff, Pause, Play, RotateCcw, Send, Trash2, UserRoundCheck, Volume2, X } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/form";
import { AiLabel, InlineAlert } from "@/components/ui/feedback";
import { Sheet } from "@/components/ui/sheet";
import { useToast } from "@/components/ui/toast";
import { aiApi } from "@/lib/api/ai.api";
import { ApiError } from "@/lib/api/errors";
import { LANG_LABEL } from "@/lib/utils/locale";
import { cn } from "@/lib/utils/cn";
import { useSpeechInput, useSpeechOutput } from "./use-speech";
import type { AssistantResponse, Lang } from "@/types";

interface ChatMsg {
  id: string;
  role: "user" | "assistant";
  text: string;
  lang: Lang;
  res?: AssistantResponse;
  failed?: ApiError | Error;
  escalated?: boolean;
}

const safeUrl = (u?: string) => (u && /^https?:\/\//i.test(u) ? u : undefined);
let seq = 0;
const nid = () => `m${++seq}`;

/** Conversation lives in component memory only: nothing is persisted in the browser. */
export function AssistantChat() {
  const t = useTranslations("assistant");
  const locale = useLocale() as Lang;
  const toast = useToast();
  const [lang, setLang] = React.useState<Lang>(locale);
  const [msgs, setMsgs] = React.useState<ChatMsg[]>([]);
  const [input, setInput] = React.useState("");
  const [pending, setPending] = React.useState(false);
  const [copied, setCopied] = React.useState<string | null>(null);
  const [disclose, setDisclose] = React.useState(false);
  const [acked, setAcked] = React.useState(false);
  const pendingRef = React.useRef(false);
  const abortRef = React.useRef<AbortController | null>(null);
  const endRef = React.useRef<HTMLDivElement>(null);

  const mic = useSpeechInput(lang, (text) => setInput((cur) => (cur ? cur + " " : "") + text));
  const tts = useSpeechOutput(lang);

  React.useEffect(() => { endRef.current?.scrollIntoView({ block: "nearest", behavior: "smooth" }); }, [msgs.length, pending]);
  React.useEffect(() => () => abortRef.current?.abort(), []);

  const ask = React.useCallback(async (question: string, retryId?: string) => {
    const q = question.trim();
    if (!q || pendingRef.current) return;
    pendingRef.current = true;
    setPending(true);
    const userId = retryId ?? nid();
    setMsgs((m) => retryId
      ? m.map((x) => (x.id === retryId ? { ...x, failed: undefined } : x))
      : [...m, { id: userId, role: "user", text: q, lang }]);
    setInput("");
    const ac = new AbortController();
    abortRef.current = ac;
    try {
      const res = await aiApi.assistant(q, lang, ac.signal);
      setMsgs((m) => [...m, { id: nid(), role: "assistant", text: res.answer, lang, res }]);
    } catch (e) {
      if (e instanceof ApiError && e.code === "ABORTED") return;
      setMsgs((m) => m.map((x) => (x.id === userId ? { ...x, failed: e as Error } : x)));
    } finally {
      pendingRef.current = false;
      setPending(false);
    }
  }, [lang]);

  const escalate = async (assistantMsg: ChatMsg) => {
    const idx = msgs.findIndex((x) => x.id === assistantMsg.id);
    const question = [...msgs.slice(0, idx)].reverse().find((x) => x.role === "user")?.text;
    if (!question) return;
    try {
      await aiApi.escalate(question);
      setMsgs((m) => m.map((x) => (x.id === assistantMsg.id ? { ...x, escalated: true } : x)));
      toast({ tone: "success", title: t("escalated") });
    } catch (e) {
      toast({ tone: "error", title: t("escalateFailed"), body: (e as Error).message });
    }
  };

  const copy = async (m: ChatMsg) => {
    try { await navigator.clipboard.writeText(m.text); setCopied(m.id); setTimeout(() => setCopied(null), 1500); }
    catch { toast({ tone: "error", title: t("copyFailed") }); }
  };

  const clear = () => { abortRef.current?.abort(); tts.stop(); setMsgs([]); setInput(""); };

  const startMic = () => { if (!acked) setDisclose(true); else mic.start(); };

  const suggestions = ["q1", "q2", "q3", "q4"] as const;
  const listening = mic.state === "listening";

  return (
    <div className="flex h-[78dvh] min-h-[28rem] flex-col overflow-hidden rounded-[32px] border-2 border-ink bg-canvas shadow-[8px_8px_0_#1c1b2b]">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b-2 border-ink bg-lavender px-4 py-3">
        <div className="flex items-center gap-2">
          <span className="grid h-9 w-9 place-items-center rounded-full border-2 border-ink bg-sun"><Bot className="h-5 w-5" aria-hidden /></span>
          <div>
            <p className="font-display text-sm font-bold">{t("name")}</p>
            <p className="text-[11px] font-medium text-ink-soft">{t("tagline")}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <label className="sr-only" htmlFor="ans-lang">{t("answerLanguage")}</label>
          <Select id="ans-lang" value={lang} onChange={(e) => { tts.stop(); setLang(e.target.value as Lang); }} className="h-9 w-auto rounded-full text-[13px]">
            {(Object.keys(LANG_LABEL) as Lang[]).map((l) => <option key={l} value={l}>{LANG_LABEL[l]}</option>)}
          </Select>
          <Button variant="secondary" size="icon-sm" aria-label={t("clear")} onClick={clear} disabled={msgs.length === 0 && !pending}><Trash2 className="h-4 w-4" aria-hidden /></Button>
        </div>
      </div>

      <div className="flex-1 space-y-4 overflow-y-auto p-4" aria-live="polite" aria-label={t("conversation")}>
        {msgs.length === 0 && (
          <div className="space-y-4">
            <InlineAlert tone="purple" title={t("welcomeTitle")}>{t("welcomeBody")}</InlineAlert>
            <p className="text-xs font-bold uppercase tracking-wider text-muted">{t("suggested")}</p>
            <div className="flex flex-wrap gap-2">
              {suggestions.map((k) => (
                <button key={k} type="button" onClick={() => ask(t(`suggest.${k}`))} className="rounded-full border-2 border-ink bg-white px-3 py-1 text-xs font-bold shadow-[2px_2px_0_#1c1b2b] transition hover:bg-sun hover:shadow-pop">
                  {t(`suggest.${k}`)}
                </button>
              ))}
            </div>
          </div>
        )}

        {msgs.map((m) => (
          <div key={m.id} className={cn("flex", m.role === "user" ? "justify-end" : "justify-start")}>
            <div className={cn("max-w-[92%] rounded-[24px] border-2 border-ink p-4 shadow-pop sm:max-w-[85%]", m.role === "user" ? "rounded-tr-sm bg-ink font-semibold text-white" : "rounded-tl-sm bg-white font-medium")}>
              <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">{m.text}</p>

              {m.failed && (
                <div className="mt-2 flex flex-wrap items-center gap-2 rounded-xl bg-danger-soft p-2 text-xs text-ink" role="alert">
                  <span className="font-semibold">{m.failed instanceof ApiError && m.failed.code === "AI_UNAVAILABLE" ? t("aiDown") : m.failed.message}</span>
                  <Button size="sm" variant="secondary" onClick={() => ask(m.text, m.id)} disabled={pending}><RotateCcw className="h-3.5 w-3.5" aria-hidden /> {t("retry")}</Button>
                </div>
              )}

              {m.role === "assistant" && m.res && (
                <div className="mt-3 space-y-3">
                  <div className="flex flex-wrap items-center gap-2"><AiLabel />{!m.res.grounded && <span className="rounded-full bg-warning-soft px-2 py-0.5 text-[11px] font-bold text-warning">{t("notGrounded")}</span>}</div>

                  {m.res.citations.length > 0 && (
                    <details className="group rounded-2xl bg-canvas p-3 text-xs">
                      <summary className="flex cursor-pointer list-none items-center justify-between gap-2 font-bold">
                        {t("sources", { count: m.res.citations.length })}
                        <ChevronDown className="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden />
                      </summary>
                      <ul className="mt-2 space-y-2">
                        {m.res.citations.map((c, i) => {
                          const href = safeUrl(c.source_url);
                          return (
                            <li key={i} className="rounded-xl bg-white p-2">
                              <p className="font-semibold">{c.source_title}</p>
                              <p className="font-mono text-[11px] text-purple-deep">{c.citation}</p>
                              {href && <a href={href} target="_blank" rel="noopener noreferrer" className="text-[11px] font-semibold underline">{t("openSource")}</a>}
                            </li>
                          );
                        })}
                      </ul>
                    </details>
                  )}

                  {(m.res.offer_escalation || m.res.is_about_own_case) && (
                    <div className="rounded-2xl bg-sun-soft p-3 text-xs">
                      <p className="font-semibold">{t("ownCase")}</p>
                      <Button size="sm" variant="outline" className="mt-2" disabled={m.escalated} onClick={() => escalate(m)}>
                        <UserRoundCheck className="h-4 w-4" aria-hidden /> {m.escalated ? t("escalated") : t("sendToWorker")}
                      </Button>
                    </div>
                  )}

                  <div className="flex flex-wrap gap-1.5">
                    <Button size="sm" variant="ghost" onClick={() => copy(m)}>{copied === m.id ? <Check className="h-4 w-4" aria-hidden /> : <Copy className="h-4 w-4" aria-hidden />} {copied === m.id ? t("copied") : t("copy")}</Button>
                    {tts.supported && (tts.speakingId === m.id
                      ? <Button size="sm" variant="ghost" onClick={tts.stop}><Pause className="h-4 w-4" aria-hidden /> {t("stopListening")}</Button>
                      : <Button size="sm" variant="ghost" onClick={() => tts.speak(m.id, m.text)}><Play className="h-4 w-4" aria-hidden /><Volume2 className="h-4 w-4" aria-hidden /> {t("listen")}</Button>)}
                  </div>
                  {tts.supported && !tts.hasVoice && <p className="text-[11px] text-muted">{t("noVoice", { lang: LANG_LABEL[lang] })}</p>}
                </div>
              )}
            </div>
          </div>
        ))}

        {pending && (
          <div className="flex justify-start" role="status">
            <div className="flex items-center gap-2 rounded-[24px] rounded-tl-sm border-2 border-ink bg-white px-4 py-3 shadow-pop">
              <span className="flex gap-1" aria-hidden>{[0, 1, 2].map((i) => <span key={i} className="h-2 w-2 rounded-full bg-purple motion-safe:animate-bounce" style={{ animationDelay: `${i * 120}ms` }} />)}</span>
              <span className="text-xs font-semibold text-muted">{t("thinking")}</span>
            </div>
          </div>
        )}
        <div ref={endRef} />
      </div>

      <div className="border-t-2 border-ink bg-white p-3">
        {mic.state === "denied" && <InlineAlert tone="warning" className="mb-2">{t("micDenied")} <button className="font-bold underline" onClick={mic.reset}>{t("dismiss")}</button></InlineAlert>}
        {mic.state === "error" && <InlineAlert tone="warning" className="mb-2">{t("micError")} <button className="font-bold underline" onClick={mic.reset}>{t("dismiss")}</button></InlineAlert>}
        {listening && (
          <div className="mb-2 flex items-center justify-between gap-2 rounded-2xl bg-pink-soft p-3 text-sm" role="status">
            <span className="flex items-center gap-2 font-semibold"><span className="h-3 w-3 rounded-full bg-danger motion-safe:animate-pulse" aria-hidden /> {t("recording")}<span className="font-normal text-ink-soft">{mic.interim}</span></span>
            <span className="flex gap-1">
              <Button size="sm" variant="primary" onClick={mic.stop}>{t("stopRecording")}</Button>
              <Button size="sm" variant="secondary" onClick={mic.cancel}><X className="h-4 w-4" aria-hidden /> {t("cancelRecording")}</Button>
            </span>
          </div>
        )}
        <form onSubmit={(e) => { e.preventDefault(); void ask(input); }} className="flex items-end gap-2">
          <div className="flex-1">
            <label htmlFor="ask" className="sr-only">{t("inputLabel")}</label>
            <textarea
              id="ask"
              rows={2}
              value={input}
              maxLength={1000}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); void ask(input); } }}
              placeholder={t("placeholder")}
              className="min-h-12 w-full resize-none rounded-2xl border-2 border-transparent bg-canvas px-4 py-2.5 text-sm font-medium shadow-clay-sm outline-none focus:border-purple"
            />
            {input && mic.supported && !listening && <p className="mt-1 text-[11px] text-muted">{t("previewHint")}</p>}
          </div>
          {mic.supported ? (
            <Button type="button" variant={listening ? "danger" : "secondary"} size="icon" aria-label={listening ? t("stopRecording") : t("startRecording")} aria-pressed={listening} onClick={listening ? mic.stop : startMic} disabled={pending}>
              {listening ? <MicOff className="h-5 w-5" aria-hidden /> : <Mic className="h-5 w-5" aria-hidden />}
            </Button>
          ) : null}
          <Button type="submit" size="icon" aria-label={t("send")} disabled={!input.trim() || pending} loading={pending}><Send className="h-5 w-5" aria-hidden /></Button>
        </form>
        {!mic.supported && <p className="mt-2 text-[11px] text-muted">{t("noMic")}</p>}
        <p className="mt-2 text-[11px] text-muted">{t("safety")}</p>
      </div>

      <Sheet
        open={disclose}
        onClose={() => setDisclose(false)}
        title={t("micTitle")}
        footer={
          <>
            <Button variant="secondary" onClick={() => setDisclose(false)}>{t("notNow")}</Button>
            <Button onClick={() => { setAcked(true); setDisclose(false); mic.start(); }}>{t("allowMic")}</Button>
          </>
        }
      >
        <p>{t("micBody")}</p>
        <ul className="mt-3 list-disc space-y-1 pl-5 text-ink-soft">
          <li>{t("micPoint1")}</li><li>{t("micPoint2")}</li><li>{t("micPoint3")}</li>
        </ul>
      </Sheet>
    </div>
  );
}
