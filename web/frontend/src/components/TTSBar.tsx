import { useState, useCallback, useEffect } from 'react';
import { api } from '../api';
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
  const [useBrowserTTS, setUseBrowserTTS] = useState(false);

  useEffect(() => {
    // Check if server TTS is available
    api.getTTSEngine().then(r => {
      if (r.engine === 'none' || r.available === 'false') {
        setUseBrowserTTS(true);
      }
    }).catch(() => setUseBrowserTTS(true));
  }, []);

  // Stop TTS when chapter changes
  useEffect(() => {
    if (speaking) {
      handleStop();
    }
  }, [chapter.id]);

  const handleSpeak = useCallback(async () => {
    const text = cleanForTTS(chapter.content);

    if (useBrowserTTS && 'speechSynthesis' in window) {
      window.speechSynthesis.cancel();
      const utt = new SpeechSynthesisUtterance(text);
      utt.rate = 0.9;
      utt.onend = () => setSpeaking(false);
      setSpeaking(true);
      window.speechSynthesis.speak(utt);
    } else {
      try {
        await api.speak(text);
        setSpeaking(true);
      } catch (e) {
        console.error('TTS error:', e);
      }
    }
  }, [chapter.content, useBrowserTTS]);

  const handleStop = useCallback(() => {
    if (useBrowserTTS && 'speechSynthesis' in window) {
      window.speechSynthesis.cancel();
    } else {
      api.stopSpeaking().catch(() => {});
    }
    setSpeaking(false);
  }, [useBrowserTTS]);

  return (
    <div className="flex items-center justify-between px-4 py-2 bg-surface border-t border-border">
      <div className="flex items-center gap-2">
        {onPrev && (
          <button
            onClick={onPrev}
            className="px-3 py-1 text-sm text-muted hover:text-text border border-border rounded hover:border-gold transition-colors"
          >
            ← Prev
          </button>
        )}
        {onNext && (
          <button
            onClick={onNext}
            className="px-3 py-1 text-sm text-muted hover:text-text border border-border rounded hover:border-gold transition-colors"
          >
            Next →
          </button>
        )}
      </div>

      <div className="flex items-center gap-2">
        {speaking ? (
          <>
            <span className="text-gold text-sm animate-pulse">🔊 Speaking…</span>
            <button
              onClick={handleStop}
              className="px-3 py-1 text-sm bg-red-900/50 text-red-300 border border-red-700 rounded hover:bg-red-900 transition-colors"
            >
              ■ Stop
            </button>
          </>
        ) : (
          <button
            onClick={handleSpeak}
            className="px-3 py-1 text-sm bg-highlight text-gold border border-gold rounded hover:bg-gold hover:text-bg transition-colors"
          >
            🔊 Speak
          </button>
        )}
      </div>
    </div>
  );
}
