import { useEffect, useRef, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { LogosService } from '../bindings';
import { findVerseForWordIndex, parseVerseSpeechSegments } from '../chapterSpeech';
import type { ChapterContent, SpeechSettings, VoiceOption } from '../types';

interface Props {
  chapter: ChapterContent;
  activeWordIndex: number;
  verseJumpToken?: number;
  verseJumpTarget?: string | null;
  onError: (message: string) => void;
  onWordIndexChange: (index: number) => void;
  onNext?: () => void;
  onPrev?: () => void;
  onRegisterWordClick?: (fn: (i: number) => void) => void;
}

function explainError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function groupVoices(voices: VoiceOption[]) {
  const order = ['piper', 'kokoro', 'say', 'windows', 'espeak', 'speechd'];
  const buckets: Record<string, VoiceOption[]> = {};

  voices.forEach((voice) => {
    if (!buckets[voice.engine]) {
      buckets[voice.engine] = [];
    }
    buckets[voice.engine].push(voice);
  });

  const groups: Array<{ engine: string; voices: VoiceOption[] }> = [];

  order.forEach((engine) => {
    if (buckets[engine]?.length) {
      groups.push({ engine, voices: buckets[engine] });
      delete buckets[engine];
    }
  });

  Object.keys(buckets)
    .sort()
    .forEach((engine) => {
      groups.push({ engine, voices: buckets[engine] });
    });

  return groups;
}

function engineLabel(engine: string) {
  switch (engine) {
    case 'speechd':
      return 'Speech Dispatcher';
    case 'espeak':
      return 'eSpeak';
    case 'windows':
      return 'Windows';
    case 'say':
      return 'macOS Say';
    default:
      return engine ? engine.charAt(0).toUpperCase() + engine.slice(1) : 'Unknown';
  }
}

export default function TTSBar({
  chapter,
  activeWordIndex,
  verseJumpToken,
  verseJumpTarget,
  onError,
  onWordIndexChange,
  onNext,
  onPrev,
  onRegisterWordClick,
}: Props) {
  const [settings, setSettings] = useState<SpeechSettings | null>(null);
  // speaking = audio is actively playing; synthesizing = waiting for synthesis to finish
  const [speaking, setSpeaking] = useState(false);
  const [synthesizing, setSynthesizing] = useState(false);
  const [paused, setPaused] = useState(false);
  const [updating, setUpdating] = useState(false);
  const timersRef = useRef<number[]>([]);
  const sessionRef = useRef(0);
  // Stores the current playback chunk so pause/resume can reschedule highlights.
  const chunkRef = useRef<{ wordOffset: number; durations: number[]; startTime: number }>({
    wordOffset: 0,
    durations: [],
    startTime: 0,
  });
  const pauseStartRef = useRef(0);
  const voiceGroups = groupVoices(settings?.voices ?? []);
  const verseSegments = parseVerseSpeechSegments(chapter.content);

  const isActive = synthesizing || speaking || paused;

  function clearHighlightTimers() {
    timersRef.current.forEach((timer) => window.clearTimeout(timer));
    timersRef.current = [];
  }

  function resetHighlightState() {
    clearHighlightTimers();
    onWordIndexChange(-1);
  }

  function clearAllPlaybackState() {
    setSynthesizing(false);
    setSpeaking(false);
    setPaused(false);
    resetHighlightState();
  }

  function beginSpeechSession() {
    sessionRef.current += 1;
    clearHighlightTimers();
    onWordIndexChange(-1);
    return sessionRef.current;
  }

  /** Schedule word-highlight timers for an entire chapter (or from startWordIndex). */
  function scheduleChapter(startWordIndex: number, durations: number[], sessionId: number) {
    clearHighlightTimers();
    chunkRef.current = { wordOffset: startWordIndex, durations, startTime: performance.now() };
    onWordIndexChange(startWordIndex);
    let elapsed = 0;
    for (let i = 1; i < durations.length; i++) {
      elapsed += Math.max(durations[i - 1] ?? 0, 1);
      const capturedI = i;
      const timer = window.setTimeout(() => {
        if (sessionRef.current !== sessionId) return;
        onWordIndexChange(startWordIndex + capturedI);
      }, elapsed);
      timersRef.current.push(timer);
    }
  }

  async function startSpeechFromWord(wordIndex: number) {
    beginSpeechSession();
    setSynthesizing(true);
    setSpeaking(false);
    setPaused(false);
    try {
      await LogosService.StartChapterPlayback(chapter.content ?? '', wordIndex);
    } catch (error) {
      clearAllPlaybackState();
      onError(explainError(error));
    }
  }

  // Register word-click handler with parent
  useEffect(() => {
    if (onRegisterWordClick) {
      onRegisterWordClick((wordIndex: number) => {
        if (isActive) {
          void startSpeechFromWord(wordIndex);
        }
      });
    }
  }, [isActive, onRegisterWordClick]);

  // TTS event subscriptions
  useEffect(() => {
    // Backend started synthesis — show spinner, keep button as "Stop"
    const unsubSynth = Events.On('tts:synthesizing', () => {
      setSynthesizing(true);
      setSpeaking(false);
    });

    // Audio started — hide spinner, schedule all word highlights
    const unsubReady = Events.On('tts:ready', (ev: any) => {
      const data = ev?.data as { startWordIndex: number; wordDurationsMs: number[] } | undefined;
      if (!data) return;
      setSynthesizing(false);
      setSpeaking(true);
      setPaused(false);
      scheduleChapter(data.startWordIndex, data.wordDurationsMs, sessionRef.current);
    });

    // Playback cancelled or error during synthesis
    const unsubDone = Events.On('tts:done', () => {
      clearAllPlaybackState();
    });

    const unsubErr = Events.On('tts:error', (ev: any) => {
      const msg = typeof ev?.data === 'string' ? ev.data : 'TTS error';
      clearAllPlaybackState();
      onError(msg);
    });

    return () => {
      unsubSynth();
      unsubReady();
      unsubDone();
      unsubErr();
    };
  }, []);

  async function refreshSettings() {
    try {
      const nextSettings = (await LogosService.GetSpeechSettings()) as SpeechSettings;
      const nextSpeaking = (await LogosService.IsSpeaking()) as boolean;
      setSettings(nextSettings);
      if (!nextSpeaking && speaking) {
        resetHighlightState();
      }
      setSpeaking(nextSpeaking);
    } catch (error) {
      onError(explainError(error));
    }
  }

  useEffect(() => {
    void refreshSettings();
  }, []);

  // Ground-truth poll — runs when audio is actively playing (not synthesizing, not paused).
  // Detects when the sox process finishes so we can clear highlight state.
  useEffect(() => {
    if (!speaking) return;

    const timer = window.setInterval(() => {
      void LogosService.IsSpeaking()
        .then((active: unknown) => {
          const nextSpeaking = Boolean(active);
          if (!nextSpeaking) {
            setSpeaking(false);
            setSynthesizing(false);
            resetHighlightState();
          }
        })
        .catch(() => undefined);
    }, 400);

    return () => window.clearInterval(timer);
  }, [speaking]);

  // Stop on chapter change
  useEffect(() => {
    sessionRef.current += 1;
    resetHighlightState();
    void LogosService.StopSpeaking()
      .catch(() => undefined)
      .finally(() => {
        setSpeaking(false);
        setSynthesizing(false);
        setPaused(false);
      });
  }, [chapter.id]);

  useEffect(() => {
    return () => {
      sessionRef.current += 1;
      resetHighlightState();
    };
  }, []);

  // Verse jump
  useEffect(() => {
    if (!verseJumpTarget || !verseJumpToken) return;
    const seg = verseSegments.find((s) => s.number === verseJumpTarget);
    void startSpeechFromWord(seg?.startWordIndex ?? 0);
  }, [verseJumpTarget, verseJumpToken]);

  async function handleSpeak() {
    if (!settings?.available) {
      onError('No supported speech engine is available. Install Kokoro or Piper, or use Windows speech / Linux eSpeak / macOS say.');
      return;
    }
    await startSpeechFromWord(0);
  }

  async function handlePause() {
    pauseStartRef.current = performance.now();
    clearHighlightTimers();
    try {
      await LogosService.PauseSpeaking();
      setSpeaking(false);
      setPaused(true);
    } catch (error) {
      onError(explainError(error));
    }
  }

  async function handleResume() {
    try {
      await LogosService.ResumeSpeaking();
      setSpeaking(true);
      setPaused(false);
      // Re-schedule remaining highlight timers adjusted for the pause gap.
      const pauseDuration = performance.now() - pauseStartRef.current;
      const { wordOffset, durations, startTime } = chunkRef.current;
      const elapsedBeforePause = pauseStartRef.current - startTime;
      const sessionId = sessionRef.current;
      let cumulativeMs = 0;
      for (let i = 1; i < durations.length; i++) {
        cumulativeMs += Math.max(durations[i - 1] ?? 0, 1);
        if (cumulativeMs > elapsedBeforePause) {
          const remaining = cumulativeMs - elapsedBeforePause;
          const capturedI = i;
          const timer = window.setTimeout(() => {
            if (sessionRef.current !== sessionId) return;
            onWordIndexChange(wordOffset + capturedI);
          }, remaining + pauseDuration);
          timersRef.current.push(timer);
        }
      }
    } catch (error) {
      onError(explainError(error));
    }
  }

  async function handleStop() {
    sessionRef.current += 1;
    clearHighlightTimers();
    setSynthesizing(false);
    setSpeaking(false);
    setPaused(false);
    onWordIndexChange(-1);
    try {
      await LogosService.StopSpeaking();
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
        {voiceGroups.length > 0 && (
          <div className="mt-3 flex flex-wrap gap-2">
            {voiceGroups.map((group) => (
              <span
                key={group.engine}
                className="rounded-full border border-border bg-surface/70 px-3 py-1 text-[0.68rem] uppercase tracking-[0.18em] text-muted"
              >
                {engineLabel(group.engine)} {group.voices.length}
              </span>
            ))}
          </div>
        )}
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
            {voiceGroups.map((group) => (
              <optgroup key={group.engine} label={engineLabel(group.engine)}>
                {group.voices.map((voice) => (
                  <option key={voice.id} value={voice.id}>
                    {voice.label}
                  </option>
                ))}
              </optgroup>
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
          {/* Primary button: Stop when active (synthesizing/speaking/paused), Read aloud otherwise */}
          <button
            type="button"
            onClick={isActive ? handleStop : handleSpeak}
            className={`flex items-center justify-center gap-2 rounded-2xl border px-4 py-3 text-sm font-semibold transition ${
              isActive
                ? 'border-red-500/40 bg-red-500/10 text-red-200 hover:bg-red-500/20'
                : 'border-gold bg-gold text-bg hover:bg-[#ffd06d]'
            }`}
          >
            {synthesizing && (
              <svg
                className="h-4 w-4 animate-spin"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                />
              </svg>
            )}
            {synthesizing ? 'Synthesizing…' : speaking ? 'Stop reading' : paused ? 'Stop' : 'Read aloud'}
          </button>

          <button
            type="button"
            onClick={speaking ? handlePause : paused ? handleResume : () => void refreshSettings()}
            disabled={synthesizing}
            className="rounded-2xl border border-border bg-bg/50 px-4 py-3 text-sm text-text transition hover:border-gold/50 hover:text-gold disabled:cursor-not-allowed disabled:opacity-40"
          >
            {speaking ? 'Pause' : paused ? 'Resume' : 'Refresh'}
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
          This uses the same shared speech stack as the CLI and TUI: Kokoro and Piper first, then macOS `say`,
          Windows speech, or Linux fallback voices when the neural engines are not installed.
        </div>
      </div>
    </section>
  );
}
