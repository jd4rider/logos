import { useEffect, useState } from 'react';
import { LogosService } from '../bindings';
import type { ChapterContent, SpeechSettings } from '../types';

interface Props {
  chapter: ChapterContent;
  onError: (message: string) => void;
  onNext?: () => void;
  onPrev?: () => void;
}

function cleanForTTS(content: string): string {
  return content
    .replace(/\[(\d+)\]/g, (_, number) => ` verse ${number}: `)
    .replace(/¶/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

function explainError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

export default function TTSBar({ chapter, onError, onNext, onPrev }: Props) {
  const [settings, setSettings] = useState<SpeechSettings | null>(null);
  const [speaking, setSpeaking] = useState(false);
  const [updating, setUpdating] = useState(false);

  async function refreshSettings() {
    try {
      const nextSettings = (await LogosService.GetSpeechSettings()) as SpeechSettings;
      const nextSpeaking = (await LogosService.IsSpeaking()) as boolean;
      setSettings(nextSettings);
      setSpeaking(nextSpeaking);
    } catch (error) {
      onError(explainError(error));
    }
  }

  useEffect(() => {
    void refreshSettings();
  }, []);

  useEffect(() => {
    if (!speaking) {
      return;
    }

    const timer = window.setInterval(() => {
      void LogosService.IsSpeaking()
        .then((active: unknown) => setSpeaking(Boolean(active)))
        .catch(() => undefined);
    }, 450);

    return () => window.clearInterval(timer);
  }, [speaking]);

  useEffect(() => {
    if (!speaking) {
      return;
    }

    void LogosService.StopSpeaking()
      .catch(() => undefined)
      .finally(() => setSpeaking(false));
  }, [chapter.id]);

  async function handleSpeak() {
    if (!settings?.available) {
      onError('No supported speech engine is available. Install Piper, Kokoro, or macOS say.');
      return;
    }

    try {
      await LogosService.SpeakText(cleanForTTS(chapter.content));
      setSpeaking(true);
    } catch (error) {
      onError(explainError(error));
    }
  }

  async function handleStop() {
    try {
      await LogosService.StopSpeaking();
      setSpeaking(false);
    } catch (error) {
      onError(explainError(error));
    }
  }

  async function handleVoiceChange(event: React.ChangeEvent<HTMLSelectElement>) {
    setUpdating(true);
    try {
      const next = (await LogosService.SetVoice(event.target.value)) as SpeechSettings;
      setSettings(next);
    } catch (error) {
      onError(explainError(error));
    } finally {
      setUpdating(false);
    }
  }

  async function handleRateChange(event: React.ChangeEvent<HTMLInputElement>) {
    setUpdating(true);
    try {
      const next = (await LogosService.SetSpeechRate(Number(event.target.value))) as SpeechSettings;
      setSettings(next);
    } catch (error) {
      onError(explainError(error));
    } finally {
      setUpdating(false);
    }
  }

  return (
    <section className="rounded-[1.75rem] border border-border/80 bg-surface/75 p-5 shadow-panel backdrop-blur-xl">
      <div className="mb-4">
        <p className="text-xs uppercase tracking-[0.24em] text-muted">Speech</p>
        <h3 className="font-display text-2xl text-text">Shared Engine</h3>
      </div>

      <div className="mb-4 rounded-[1.35rem] border border-border bg-bg/45 px-4 py-3">
        <div className="mb-1 text-xs uppercase tracking-[0.22em] text-muted">Active backend</div>
        <div className="text-sm text-text">
          {settings?.available ? settings.activeVoice?.label ?? settings.engine : 'Unavailable'}
        </div>
      </div>

      <div className="space-y-4">
        <label className="block">
          <span className="mb-2 block text-xs uppercase tracking-[0.22em] text-muted">Voice</span>
          <select
            value={settings?.activeVoice?.id ?? ''}
            onChange={handleVoiceChange}
            disabled={!settings?.available || updating}
            className="w-full rounded-2xl border border-border bg-bg/60 px-4 py-3 text-sm text-text outline-none transition focus:border-gold disabled:opacity-60"
          >
            {(settings?.voices ?? []).map((voice) => (
              <option key={voice.id} value={voice.id}>
                {voice.label}
              </option>
            ))}
          </select>
        </label>

        <label className="block">
          <div className="mb-2 flex items-center justify-between gap-3">
            <span className="text-xs uppercase tracking-[0.22em] text-muted">Rate</span>
            <span className="text-sm text-text">{settings?.rate ?? 150} wpm</span>
          </div>
          <input
            type="range"
            min="80"
            max="260"
            step="5"
            value={settings?.rate ?? 150}
            onChange={handleRateChange}
            disabled={!settings?.available || updating}
            className="w-full accent-[#f5bf52]"
          />
        </label>

        <div className="grid grid-cols-2 gap-3">
          <button
            type="button"
            onClick={speaking ? handleStop : handleSpeak}
            className={`rounded-2xl border px-4 py-3 text-sm font-semibold transition ${
              speaking
                ? 'border-red-500/40 bg-red-500/10 text-red-200 hover:bg-red-500/20'
                : 'border-gold bg-gold text-bg hover:bg-[#ffd06d]'
            }`}
          >
            {speaking ? 'Stop reading' : 'Read aloud'}
          </button>

          <button
            type="button"
            onClick={() => void refreshSettings()}
            className="rounded-2xl border border-border bg-bg/50 px-4 py-3 text-sm text-text transition hover:border-gold/50 hover:text-gold"
          >
            Refresh
          </button>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <button
            type="button"
            onClick={onPrev}
            disabled={!onPrev}
            className="rounded-2xl border border-border bg-bg/45 px-4 py-3 text-sm text-text transition hover:border-gold/50 hover:text-gold disabled:cursor-not-allowed disabled:opacity-40"
          >
            Previous
          </button>
          <button
            type="button"
            onClick={onNext}
            disabled={!onNext}
            className="rounded-2xl border border-border bg-bg/45 px-4 py-3 text-sm text-text transition hover:border-gold/50 hover:text-gold disabled:cursor-not-allowed disabled:opacity-40"
          >
            Next
          </button>
        </div>

        <div className="rounded-[1.35rem] border border-border bg-bg/35 px-4 py-3 text-sm text-muted">
          This uses the same Piper, Kokoro, or macOS `say` backend stack the CLI/TUI uses.
        </div>
      </div>
    </section>
  );
}
