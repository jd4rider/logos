import { useState, useCallback, useEffect, useRef } from 'react';
import {
  SpeakSynced, StopSpeaking, PauseSpeaking, ResumeSpeaking,
  IsPaused, IsSpeaking, ListVoices, GetActiveVoice, SetVoice,
  GetTTSEngine, GetTTSRate, SetTTSRate
} from '../wailsjs/go/main/App';
import type { ChapterContent, VoiceEntry } from '../types';

interface Props {
  chapter: ChapterContent;
  onNext?: () => void;
  onPrev?: () => void;
  onWordIndex: (idx: number) => void;
}

function cleanForTTS(content: string): string {
  return content
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/\*{1,3}([^*\n]+)\*{1,3}/g, '$1')
    .replace(/_{1,2}([^_\n]+)_{1,2}/g, '$1')
    .replace(/\[(\d+)\]/g, '... ')
    .replace(/¶/g, '')
    .replace(/\s{2,}/g, ' ')
    .trim();
}

export default function TTSBar({ chapter, onNext, onPrev, onWordIndex }: Props) {
  const [speaking, setSpeaking] = useState(false);
  const [paused, setPaused] = useState(false);
  const [engine, setEngine] = useState('');
  const [voices, setVoices] = useState<VoiceEntry[]>([]);
  const [activeVoice, setActiveVoice] = useState<VoiceEntry | null>(null);
  const [rate, setRate] = useState(150);
  const [showVoicePicker, setShowVoicePicker] = useState(false);

  const ttsSessionRef = useRef(0);
  const timeoutIdsRef = useRef<ReturnType<typeof setTimeout>[]>([]);
  const pauseTimeRef = useRef<number | null>(null);
  const pauseRemainingRef = useRef<{ idx: number; remaining: number }[]>([]);

  function clearTimeouts() {
    timeoutIdsRef.current.forEach(clearTimeout);
    timeoutIdsRef.current = [];
  }

  useEffect(() => {
    GetTTSEngine().then(setEngine).catch(() => {});
    ListVoices().then(setVoices).catch(() => {});
    GetActiveVoice().then(setActiveVoice).catch(() => {});
    GetTTSRate().then(setRate).catch(() => {});
  }, []);

  useEffect(() => {
    if (speaking) handleStop();
  }, [chapter.id]);

  const handleSpeak = useCallback(async () => {
    clearTimeouts();
    ttsSessionRef.current++;
    const session = ttsSessionRef.current;
    setSpeaking(true);
    setPaused(false);
    pauseRemainingRef.current = [];

    const text = cleanForTTS(chapter.content);
    try {
      const durs = await SpeakSynced(text);
      if (ttsSessionRef.current !== session) return;

      let elapsed = 0;
      durs.forEach((dur, idx) => {
        const id = setTimeout(() => {
          if (ttsSessionRef.current !== session) return;
          onWordIndex(idx);
          if (idx === durs.length - 1) {
            setSpeaking(false);
            setPaused(false);
            onWordIndex(-1);
          }
        }, elapsed);
        timeoutIdsRef.current.push(id);
        elapsed += dur;
      });
    } catch {
      setSpeaking(false);
    }
  }, [chapter.content, onWordIndex]);

  const handleStop = useCallback(() => {
    clearTimeouts();
    ttsSessionRef.current++;
    StopSpeaking().catch(() => {});
    setSpeaking(false);
    setPaused(false);
    onWordIndex(-1);
  }, [onWordIndex]);

  const handlePause = useCallback(() => {
    PauseSpeaking().catch(() => {});
    setPaused(true);
    pauseTimeRef.current = Date.now();
    clearTimeouts();
  }, []);

  const handleResume = useCallback(() => {
    ResumeSpeaking().catch(() => {});
    setPaused(false);
  }, []);

  const handleVoiceChange = useCallback(async (voiceId: string) => {
    await SetVoice(voiceId);
    const updated = await GetActiveVoice();
    setActiveVoice(updated);
  }, []);

  const handleRateChange = useCallback(async (newRate: number) => {
    setRate(newRate);
    await SetTTSRate(newRate);
  }, []);

  return (
    <div className="flex flex-col bg-surface border-t border-border">
      <div className="flex items-center justify-between px-4 py-2">
        {/* Left: prev/next */}
        <div className="flex gap-2">
          {onPrev && (
            <button onClick={onPrev} className="px-3 py-1 text-sm text-muted hover:text-text border border-border rounded hover:border-gold transition-colors">
              ← Prev
            </button>
          )}
          {onNext && (
            <button onClick={onNext} className="px-3 py-1 text-sm text-muted hover:text-text border border-border rounded hover:border-gold transition-colors">
              Next →
            </button>
          )}
        </div>

        {/* Center: speak/pause/stop */}
        <div className="flex items-center gap-2">
          {!speaking ? (
            <button onClick={handleSpeak} className="px-3 py-1 text-sm bg-highlight text-gold border border-gold rounded hover:bg-gold hover:text-bg transition-colors">
              🔊 Speak
            </button>
          ) : (
            <>
              <span className="text-gold text-sm animate-pulse">🔊 {paused ? 'Paused' : 'Speaking…'}</span>
              {!paused ? (
                <button onClick={handlePause} className="px-3 py-1 text-sm bg-highlight text-accent border border-border rounded hover:border-gold transition-colors">
                  ⏸ Pause
                </button>
              ) : (
                <button onClick={handleResume} className="px-3 py-1 text-sm bg-highlight text-accent border border-border rounded hover:border-gold transition-colors">
                  ▶ Resume
                </button>
              )}
              <button onClick={handleStop} className="px-3 py-1 text-sm bg-red-900/50 text-red-300 border border-red-700 rounded hover:bg-red-900">
                ■ Stop
              </button>
            </>
          )}
          {activeVoice && (
            <span className="text-xs text-muted">{activeVoice.name}</span>
          )}
          {engine && engine !== 'none' && (
            <span className="text-xs text-muted opacity-60">{engine}</span>
          )}
        </div>

        {/* Right: settings toggle */}
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowVoicePicker(!showVoicePicker)}
            className={`px-2 py-1 text-xs border rounded transition-colors ${showVoicePicker ? 'border-gold text-gold' : 'border-border text-muted hover:border-gold hover:text-text'}`}
          >
            ⚙ Voice
          </button>
        </div>
      </div>

      {/* Voice / rate panel */}
      {showVoicePicker && (
        <div className="px-4 pb-3 flex flex-wrap items-center gap-4 border-t border-border/50 pt-2">
          {voices.length > 0 && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted">Voice:</span>
              <select
                value={activeVoice?.id ?? ''}
                onChange={e => handleVoiceChange(e.target.value)}
                className="text-xs bg-bg border border-border rounded px-2 py-1 text-text"
              >
                {voices.map(v => (
                  <option key={v.id} value={v.id}>{v.name}</option>
                ))}
              </select>
            </div>
          )}
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted">Rate: {rate} WPM</span>
            <input
              type="range"
              min={50}
              max={300}
              step={10}
              value={rate}
              onChange={e => handleRateChange(Number(e.target.value))}
              className="w-28 accent-gold"
            />
          </div>
        </div>
      )}
    </div>
  );
}
