"use client";

import * as React from "react";
import { SPEECH_LOCALE } from "@/lib/utils/locale";
import type { Lang } from "@/types";

/* Minimal typings for the browser-native Web Speech API (not in lib.dom for all targets). */
interface SRAlternative { transcript: string }
interface SRResult { 0: SRAlternative; isFinal: boolean; length: number }
interface SREvent { resultIndex: number; results: ArrayLike<SRResult> }
interface SRErrorEvent { error: string }
interface SpeechRec {
  lang: string; continuous: boolean; interimResults: boolean;
  start(): void; stop(): void; abort(): void;
  onresult: ((e: SREvent) => void) | null; onerror: ((e: SRErrorEvent) => void) | null; onend: (() => void) | null;
}
type SRCtor = new () => SpeechRec;

function getCtor(): SRCtor | null {
  if (typeof window === "undefined") return null;
  const w = window as unknown as { SpeechRecognition?: SRCtor; webkitSpeechRecognition?: SRCtor };
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null;
}

export type MicState = "idle" | "listening" | "denied" | "error";

/**
 * Progressive-enhancement voice input using the browser's own speech recognition.
 * Note: in Chromium-based browsers the audio is processed by the browser vendor's service; the app itself
 * never receives or uploads audio. The UI discloses this before first use.
 */
export function useSpeechInput(lang: Lang, onFinal: (text: string) => void) {
  const supported = React.useMemo(() => getCtor() !== null, []);
  const [state, setState] = React.useState<MicState>("idle");
  const [interim, setInterim] = React.useState("");
  const rec = React.useRef<SpeechRec | null>(null);
  const cancelled = React.useRef(false);
  const finalText = React.useRef("");
  const cb = React.useRef(onFinal);
  React.useEffect(() => { cb.current = onFinal; });

  const stopAll = React.useCallback(() => { rec.current?.abort(); rec.current = null; }, []);
  React.useEffect(() => stopAll, [stopAll]);

  const start = React.useCallback(() => {
    const Ctor = getCtor();
    if (!Ctor || rec.current) return;
    const r = new Ctor();
    r.lang = SPEECH_LOCALE[lang];
    r.continuous = true;
    r.interimResults = true;
    cancelled.current = false;
    finalText.current = "";
    setInterim("");
    r.onresult = (e) => {
      let interimText = "";
      for (let i = e.resultIndex; i < e.results.length; i++) {
        const res = e.results[i];
        if (res.isFinal) finalText.current += res[0].transcript + " ";
        else interimText += res[0].transcript;
      }
      setInterim(interimText);
    };
    r.onerror = (e) => {
      if (e.error === "not-allowed" || e.error === "service-not-allowed") setState("denied");
      else if (e.error !== "aborted" && e.error !== "no-speech") setState("error");
    };
    r.onend = () => {
      rec.current = null;
      setState((s) => (s === "denied" || s === "error" ? s : "idle"));
      const text = (finalText.current + " " + "").trim();
      if (!cancelled.current && text) cb.current(text); // placed in the input as a preview; never auto-sent
      setInterim("");
    };
    rec.current = r;
    setState("listening");
    try { r.start(); } catch { setState("error"); rec.current = null; }
  }, [lang]);

  const stop = React.useCallback(() => rec.current?.stop(), []);
  const cancel = React.useCallback(() => { cancelled.current = true; rec.current?.abort(); }, []);
  const reset = React.useCallback(() => setState("idle"), []);

  return { supported, state, interim, start, stop, cancel, reset };
}

/** Text-to-speech via speechSynthesis. Reports whether a voice for the language exists. */
export function useSpeechOutput(lang: Lang) {
  const supported = typeof window !== "undefined" && "speechSynthesis" in window;
  const [speakingId, setSpeakingId] = React.useState<string | null>(null);
  const [hasVoice, setHasVoice] = React.useState(true);

  React.useEffect(() => {
    if (!supported) return;
    const check = () => {
      const tag = SPEECH_LOCALE[lang].toLowerCase();
      const base = tag.split("-")[0];
      const voices = window.speechSynthesis.getVoices();
      setHasVoice(voices.length === 0 ? true : voices.some((v) => v.lang.toLowerCase().startsWith(base)));
    };
    check();
    window.speechSynthesis.addEventListener("voiceschanged", check);
    return () => window.speechSynthesis.removeEventListener("voiceschanged", check);
  }, [lang, supported]);

  React.useEffect(() => () => { if (supported) window.speechSynthesis.cancel(); }, [supported]);

  const speak = React.useCallback((id: string, text: string) => {
    if (!supported) return;
    window.speechSynthesis.cancel();
    const u = new SpeechSynthesisUtterance(text);
    u.lang = SPEECH_LOCALE[lang];
    u.onend = () => setSpeakingId((s) => (s === id ? null : s));
    u.onerror = () => setSpeakingId((s) => (s === id ? null : s));
    setSpeakingId(id);
    window.speechSynthesis.speak(u);
  }, [lang, supported]);

  const stop = React.useCallback(() => { if (supported) window.speechSynthesis.cancel(); setSpeakingId(null); }, [supported]);

  return { supported, hasVoice, speakingId, speak, stop };
}
