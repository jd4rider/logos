import { useState, useEffect, useRef, useCallback } from 'react';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import {
  StartAIStream, StopAIStream, IsAIAvailable,
  SaveToLibrary, ListLibrary, ExportPDF
} from '../wailsjs/go/main/App';
import type { LibraryEntry, AIAction } from '../types';

interface Props {
  verseRef: string;
  verseText: string;
  chapterText: string;
  bookName: string;
  chapterNum: string;
  translation: string;
  onClose: () => void;
}

function renderMarkdown(text: string): string {
  return text
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    .replace(/^### (.+)$/gm, '<h3 class="text-gold font-bold mt-3 mb-1">$1</h3>')
    .replace(/^## (.+)$/gm, '<h2 class="text-gold font-bold mt-4 mb-2">$1</h2>')
    .replace(/^# (.+)$/gm, '<h1 class="text-gold font-bold mt-4 mb-2 text-lg">$1</h1>')
    .replace(/\n\n/g, '</p><p class="mb-3">')
    .replace(/\n/g, '<br/>');
}

type View = 'menu' | 'typing' | 'streaming' | 'result' | 'library';

export default function AIPanel({ verseRef, verseText, chapterText, bookName, chapterNum, translation, onClose }: Props) {
  const [view, setView] = useState<View>('menu');
  const [streamText, setStreamText] = useState('');
  const [resultText, setResultText] = useState('');
  const [currentAction, setCurrentAction] = useState<AIAction>('explain_verse');
  const [userInput, setUserInput] = useState('');
  const [aiAvailable, setAIAvailable] = useState(false);
  const [library, setLibrary] = useState<LibraryEntry[]>([]);
  const [selectedEntry, setSelectedEntry] = useState<LibraryEntry | null>(null);
  const [saveStatus, setSaveStatus] = useState('');
  const [exportStatus, setExportStatus] = useState('');
  const streamRef = useRef('');
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    IsAIAvailable().then(setAIAvailable).catch(() => setAIAvailable(false));
  }, []);

  useEffect(() => {
    const onToken = (token: string) => {
      streamRef.current += token;
      setStreamText(streamRef.current);
      scrollRef.current?.scrollTo(0, scrollRef.current.scrollHeight);
    };
    const onDone = () => {
      setResultText(streamRef.current);
      setView('result');
    };
    const onError = (err: string) => {
      setResultText('Error: ' + err);
      setView('result');
    };

    EventsOn('ai:token', onToken);
    EventsOn('ai:done', onDone);
    EventsOn('ai:error', onError);
    return () => {
      EventsOff('ai:token');
      EventsOff('ai:done');
      EventsOff('ai:error');
    };
  }, []);

  const startStream = useCallback((action: AIAction, input = '') => {
    streamRef.current = '';
    setStreamText('');
    setView('streaming');
    StartAIStream(action, verseRef, verseText, chapterText, bookName, chapterNum, translation, input);
  }, [verseRef, verseText, chapterText, bookName, chapterNum, translation]);

  const handleMenuAction = (action: AIAction) => {
    if (action === 'library') {
      setView('library');
      ListLibrary().then(setLibrary).catch(() => {});
      return;
    }
    if (action === 'ask') {
      setCurrentAction('ask');
      setView('typing');
      return;
    }
    setCurrentAction(action);
    startStream(action);
  };

  const handleSave = async () => {
    const model = 'ollama';
    const kindMap: Record<string, string> = {
      explain_verse: 'note', explain_chapter: 'note',
      devotional: 'devotional', sermon: 'sermon', ask: 'note',
    };
    const kind = kindMap[currentAction] ?? 'note';
    const title = `${currentAction.replace('_', ' ')} - ${verseRef}`;
    try {
      await SaveToLibrary(kind, title, verseRef, resultText, model);
      setSaveStatus('Saved!');
      setTimeout(() => setSaveStatus(''), 2000);
    } catch (e) {
      setSaveStatus('Error: ' + String(e));
    }
  };

  const handleExport = async () => {
    const kindMap: Record<string, string> = {
      devotional: 'devotional', sermon: 'sermon',
    };
    const kind = kindMap[currentAction] ?? 'note';
    const title = `${currentAction.replace('_', ' ')} - ${verseRef}`;
    try {
      const path = await ExportPDF(kind, title, verseRef, resultText);
      setExportStatus('Exported to ' + path);
      setTimeout(() => setExportStatus(''), 3000);
    } catch (e) {
      setExportStatus('Error: ' + String(e));
    }
  };

  const menuItems: { action: AIAction; label: string; icon: string }[] = [
    { action: 'explain_verse', label: 'Explain Verse', icon: '📖' },
    { action: 'explain_chapter', label: 'Explain Chapter', icon: '📚' },
    { action: 'devotional', label: 'Generate Devotional', icon: '🙏' },
    { action: 'sermon', label: 'Generate Sermon', icon: '✝' },
    { action: 'ask', label: 'Ask a Question', icon: '💬' },
    { action: 'library', label: 'My Library', icon: '🗂' },
  ];

  return (
    <div className="flex flex-col h-full bg-surface border-l border-border w-80 flex-shrink-0">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border">
        <span className="text-gold font-semibold text-sm uppercase tracking-wider">✝ AI Assistant</span>
        <div className="flex items-center gap-2">
          {view !== 'menu' && (
            <button
              onClick={() => { StopAIStream(); setView('menu'); }}
              className="text-xs text-muted hover:text-text"
            >
              ← Menu
            </button>
          )}
          <button onClick={onClose} className="text-muted hover:text-text text-lg leading-none">×</button>
        </div>
      </div>

      {/* Verse context */}
      {verseRef && (
        <div className="px-4 py-2 bg-highlight/30 border-b border-border/50">
          <div className="text-xs text-gold font-medium">{verseRef}</div>
          {verseText && <div className="text-xs text-muted mt-1 line-clamp-2">{verseText}</div>}
        </div>
      )}

      {!aiAvailable && (
        <div className="px-4 py-2 text-xs text-amber-400 bg-amber-900/20 border-b border-amber-700/30">
          ⚠ Ollama not detected. AI features require Ollama running locally.
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto" ref={scrollRef}>
        {view === 'menu' && (
          <div className="p-3 space-y-2">
            {menuItems.map(item => (
              <button
                key={item.action}
                onClick={() => handleMenuAction(item.action)}
                className="w-full text-left px-3 py-2 rounded border border-border hover:border-gold hover:bg-highlight/50 transition-colors flex items-center gap-2"
              >
                <span>{item.icon}</span>
                <span className="text-sm text-text">{item.label}</span>
              </button>
            ))}
          </div>
        )}

        {view === 'typing' && (
          <div className="p-4 space-y-3">
            <p className="text-xs text-muted">Ask a question about this verse or chapter:</p>
            <textarea
              value={userInput}
              onChange={e => setUserInput(e.target.value)}
              placeholder="What does this verse mean in context?"
              className="w-full h-24 bg-bg border border-border rounded p-2 text-sm text-text resize-none focus:border-gold outline-none"
            />
            <div className="flex gap-2">
              <button
                onClick={() => { startStream('ask', userInput); }}
                disabled={!userInput.trim()}
                className="flex-1 px-3 py-2 text-sm bg-highlight text-gold border border-gold rounded hover:bg-gold hover:text-bg transition-colors disabled:opacity-50"
              >
                Ask
              </button>
              <button onClick={() => setView('menu')} className="px-3 py-2 text-sm text-muted border border-border rounded hover:text-text">
                Cancel
              </button>
            </div>
          </div>
        )}

        {view === 'streaming' && (
          <div className="p-4">
            <div className="flex items-center gap-2 mb-3">
              <span className="text-gold text-sm animate-pulse">⟳ Generating…</span>
              <button onClick={() => { StopAIStream(); setView('menu'); }} className="text-xs text-muted hover:text-red-400">Stop</button>
            </div>
            <div className="text-sm text-text leading-relaxed whitespace-pre-wrap font-sans">{streamText}</div>
          </div>
        )}

        {view === 'result' && (
          <div className="p-4">
            <div
              className="text-sm text-text leading-relaxed prose-sm"
              dangerouslySetInnerHTML={{ __html: '<p class="mb-3">' + renderMarkdown(resultText) + '</p>' }}
            />
            <div className="flex flex-wrap gap-2 mt-4 pt-3 border-t border-border">
              <button onClick={handleSave} className="px-3 py-1 text-xs bg-highlight border border-border rounded hover:border-gold text-text">
                �� Save
              </button>
              <button onClick={handleExport} className="px-3 py-1 text-xs bg-highlight border border-border rounded hover:border-gold text-text">
                📄 Export PDF
              </button>
            </div>
            {saveStatus && <p className="text-xs text-green-400 mt-2">{saveStatus}</p>}
            {exportStatus && <p className="text-xs text-green-400 mt-2">{exportStatus}</p>}
          </div>
        )}

        {view === 'library' && (
          <div className="p-3">
            {selectedEntry ? (
              <div>
                <button onClick={() => setSelectedEntry(null)} className="text-xs text-muted hover:text-text mb-2">← Back to list</button>
                <div className="text-xs text-gold font-medium mb-1">{selectedEntry.title}</div>
                <div className="text-xs text-muted mb-2">{selectedEntry.ref} · {selectedEntry.date}</div>
                <div className="text-sm text-text leading-relaxed whitespace-pre-wrap">{selectedEntry.content}</div>
              </div>
            ) : (
              <>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs text-gold uppercase tracking-wider">Saved Items</span>
                  <button onClick={() => ListLibrary().then(setLibrary).catch(() => {})} className="text-xs text-muted hover:text-text">↻ Refresh</button>
                </div>
                {library.length === 0 ? (
                  <p className="text-xs text-muted">No saved items yet.</p>
                ) : (
                  <div className="space-y-2">
                    {library.map(e => (
                      <button
                        key={`${e.kind}-${e.id}`}
                        onClick={() => setSelectedEntry(e)}
                        className="w-full text-left p-2 rounded border border-border hover:border-gold transition-colors"
                      >
                        <div className="flex items-center justify-between">
                          <span className="text-xs uppercase text-muted">{e.kind}</span>
                          <span className="text-xs text-muted">{e.date}</span>
                        </div>
                        <div className="text-sm text-text truncate mt-0.5">{e.title}</div>
                        <div className="text-xs text-muted">{e.ref}</div>
                      </button>
                    ))}
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
