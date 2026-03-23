import { useState, useCallback, useEffect } from 'react';
import { SpeakText, StopSpeaking, GetTTSEngine } from '../wailsjs/go/main/App';
import type { ChapterContent } from '../types';

interface Props {
  chapter: ChapterContent;
  onNext?: () => void;
  onPrev?: () => void;
}

function cleanForTTS(content: string): string {
  return content
    .replace(/\[(\d+)\]/g, (_, n) => ` verse ${n}: `)
    .replace(/¶/g, '')
    .replace(/\s+/g, ' ')
    .trim();
}

export default function TTSBar({ chapter, onNext, onPrev }: Props) {
  const [speaking, setSpeaking] = useState(false);
  const [engine, setEngine] = useState('');

  useEffect(() => {
    GetTTSEngine().then(setEngine).catch(() => {});
  }, []);

  useEffect(() => {
    if (speaking) handleStop();
  }, [chapter.id]);

  const handleSpeak = useCallback(async () => {
    const text = cleanForTTS(chapter.content);
    if (engine === 'none') {
      // Browser TTS fallback
      if ('speechSynthesis' in window) {
        window.speechSynthesis.cancel();
        const utt = new SpeechSynthesisUtterance(text);
        utt.rate = 0.9;
        utt.onend = () => setSpeaking(false);
        setSpeaking(true);
        window.speechSynthesis.speak(utt);
      }
    } else {
      try {
        await SpeakText(text);
        setSpeaking(true);
      } catch (e) {
        console.error('TTS error:', e);
      }
    }
  }, [chapter.content, engine]);

  const handleStop = useCallback(() => {
    if (engine === 'none' && 'speechSynthesis' in window) {
      window.speechSynthesis.cancel();
    } else {
      StopSpeaking().catch(() => {});
    }
    setSpeaking(false);
  }, [engine]);

  return (
    <div className="flex items-center justify-between px-4 py-2 bg-surface border-t border-border">
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
      <div className="flex items-center gap-2">
        {speaking ? (
          <>
            <span className="text-gold text-sm animate-pulse">🔊 Speaking…</span>
            <button onClick={handleStop} className="px-3 py-1 text-sm bg-red-900/50 text-red-300 border border-red-700 rounded hover:bg-red-900">
              ■ Stop
            </button>
          </>
        ) : (
          <button onClick={handleSpeak} className="px-3 py-1 text-sm bg-highlight text-gold border border-gold rounded hover:bg-gold hover:text-bg transition-colors">
            🔊 Speak
          </button>
        )}
        {engine && engine !== 'none' && (
          <span className="text-xs text-muted">{engine}</span>
        )}
      </div>
    </div>
  );
}
