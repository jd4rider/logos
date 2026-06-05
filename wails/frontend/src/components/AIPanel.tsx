import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { Events } from '@wailsio/runtime';
import { LogosService } from '../bindings';
import type { AIAction, LibraryEntry, SyncedSpeechPlan } from '../types';
import LocalSetupCard from './LocalSetupCard';

const SLASH_COMMANDS = [
  { cmd: '/devotional', description: 'Write a devotional for this passage' },
  { cmd: '/sermon',     description: 'Build a sermon outline for this passage' },
  { cmd: '/explain',    description: 'Get a pastoral overview of this passage' },
  { cmd: '/save',       description: 'Save the current output to your library' },
  { cmd: '/pdf',        description: 'Export the current output as a PDF' },
] as const;

type SlashCmd = typeof SLASH_COMMANDS[number]['cmd'];

interface Props {
  mode: 'chat' | 'tools';
  verseRef: string;
  verseText: string;
  chapterText: string;
  bookName: string;
  chapterNum: string;
  translation: string;
  onClose: () => void;
}

interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  label?: string;
  action?: AIAction | 'chat';
}

type View = 'chat' | 'library';

function stripMarkdown(text: string): string {
  return text
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/\*([^*]+)\*/g, '$1')
    .replace(/^#+\s+/gm, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
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

function createMessageId() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function buildConversationPrompt(messages: ChatMessage[]) {
  const history = messages
    .slice(-8)
    .map((message) => `${message.role === 'user' ? 'User' : 'Pastor'}: ${stripMarkdown(message.content)}`)
    .join('\n\n');

  return history;
}

function ThinkingSpinner() {
  return (
    <span className="inline-flex items-center gap-2">
      <svg className="h-4 w-4 animate-spin text-gold" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <circle cx="12" cy="12" r="10" className="opacity-20" stroke="currentColor" strokeWidth="4" />
        <path
          className="opacity-90"
          fill="currentColor"
          d="M12 2a10 10 0 0 1 10 10h-4a6 6 0 0 0-6-6V2z"
        />
      </svg>
      <span>Thinking...</span>
    </span>
  );
}

function speakerName(role: ChatMessage['role']) {
  return role === 'user' ? 'User' : 'Logos';
}

function outputKindForAction(action: AIAction | 'chat' | null) {
  if (action === 'devotional') {
    return 'devotional';
  }
  if (action === 'sermon') {
    return 'sermon';
  }
  return 'note';
}

function outputTitleForAction(action: AIAction | 'chat' | null, verseRef: string) {
  switch (action) {
    case 'explain_chapter':
      return `Passage notes - ${verseRef}`;
    case 'devotional':
      return `Devotional - ${verseRef}`;
    case 'sermon':
      return `Sermon outline - ${verseRef}`;
    case 'chat':
    case 'ask':
      return `Logos Chat - ${verseRef}`;
    default:
      return `Study note - ${verseRef}`;
  }
}

function WordHighlight({ text, activeWord }: { text: string; activeWord: number }) {
  const words = text.split(/\s+/).filter(Boolean);
  return (
    <p className="whitespace-pre-wrap text-sm leading-8 text-text">
      {words.map((word, i) => (
        <span key={i}>
          {i > 0 && ' '}
          <span
            className={
              i === activeWord
                ? 'rounded-md bg-gold/20 px-1 py-0.5 text-gold shadow-[0_0_0_1px_rgba(245,191,82,0.35)]'
                : ''
            }
          >
            {word}
          </span>
        </span>
      ))}
    </p>
  );
}

export default function AIPanel({
  mode,
  verseRef,
  verseText,
  chapterText,
  bookName,
  chapterNum,
  translation,
  onClose,
}: Props) {
  const [view, setView] = useState<View>('chat');
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [chatInput, setChatInput] = useState('');
  const [streaming, setStreaming] = useState(false);
  const [streamingText, setStreamingText] = useState('');
  const [currentAction, setCurrentAction] = useState<AIAction | 'chat' | null>(null);
  const [currentLabel, setCurrentLabel] = useState('Logos Chat');
  const [selectedOutput, setSelectedOutput] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [library, setLibrary] = useState<LibraryEntry[]>([]);
  const [selectedEntry, setSelectedEntry] = useState<LibraryEntry | null>(null);
  const [aiAvailable, setAiAvailable] = useState(false);
  const [speaking, setSpeaking] = useState(false);
  const [starting, setStarting] = useState(false);
  const [activeWord, setActiveWord] = useState(-1);
  const [precaching, setPrecaching] = useState(false);
  const [isSaved, setIsSaved] = useState(false);
  const [savedToast, setSavedToast] = useState(false);
  const [slashMenuOpen, setSlashMenuOpen] = useState(false);
  const [slashMenuIndex, setSlashMenuIndex] = useState(0);
  const [slashFilter, setSlashFilter] = useState('');

  const sessionRef = useRef(0);
  const timersRef = useRef<number[]>([]);
  const transcriptRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const chatMessagesRef = useRef<ChatMessage[]>([]);
  const streamingTextRef = useRef('');
  const modeRef = useRef(mode);
  const currentActionRef = useRef<AIAction | 'chat' | null>(null);
  const currentLabelRef = useRef('Logos Chat');
  const hasCommittedRef = useRef(false);
  const savedToastTimerRef = useRef<number | undefined>(undefined);
  const verseRefRef = useRef(verseRef);

  function clearHighlightTimers() {
    timersRef.current.forEach((timer) => window.clearTimeout(timer));
    timersRef.current = [];
  }

  function stopReadingState() {
    sessionRef.current += 1;
    clearHighlightTimers();
    setActiveWord(-1);
    setSpeaking(false);
    setStarting(false);
  }

  function scheduleHighlights(plan: SyncedSpeechPlan, sessionId: number) {
    clearHighlightTimers();
    const durations = plan.wordDurationsMs ?? [];
    if (durations.length === 0) {
      setActiveWord(-1);
      return;
    }

    setActiveWord(0);
    let elapsed = 0;
    for (let i = 1; i < durations.length; i++) {
      elapsed += Math.max(durations[i - 1] ?? 0, 1);
      const captured = i;
      const timer = window.setTimeout(() => {
        if (sessionRef.current !== sessionId) {
          return;
        }
        setActiveWord(captured);
      }, elapsed);
      timersRef.current.push(timer);
    }
  }

  function commitStreamingMessage(reason: 'done' | 'stopped') {
    if (hasCommittedRef.current) return;
    hasCommittedRef.current = true;

    const finalText = streamingTextRef.current.trim();
    const action = currentActionRef.current;
    if (!finalText) {
      setStreaming(false);
      setStreamingText('');
      streamingTextRef.current = '';
      return;
    }

    const message: ChatMessage = {
      id: createMessageId(),
      role: 'assistant',
      content: finalText,
      label: currentLabelRef.current,
      action: action ?? 'chat',
    };

    if (modeRef.current === 'chat') {
      setChatMessages((previous) => {
        const next = [...previous, message];
        chatMessagesRef.current = next;
        return next;
      });
    }
    setSelectedOutput(finalText);
    setIsSaved(false);
    setPrecaching(true);
    setStreaming(false);
    setStreamingText('');
    streamingTextRef.current = '';
    void LogosService.StartAIPrecache(finalText).then(() => setPrecaching(false)).catch(() => setPrecaching(false));

    if (reason === 'done' && (action === 'devotional' || action === 'sermon')) {
      void autoSaveOutput(action, finalText, verseRefRef.current);
    }
  }

  useEffect(() => {
    void LogosService.IsAIAvailable().then((value: unknown) => setAiAvailable(value as boolean));
  }, []);

  useEffect(() => {
    modeRef.current = mode;
  }, [mode]);

  useEffect(() => {
    verseRefRef.current = verseRef;
  }, [verseRef]);

  useEffect(() => {
    const unsubToken = Events.On('ai:token', (event) => {
      streamingTextRef.current += event.data as string;
      setStreamingText(streamingTextRef.current);
    });
    const unsubDone = Events.On('ai:done', () => {
      commitStreamingMessage('done');
    });
    const unsubError = Events.On('ai:error', (event) => {
      setError(event.data as string);
      setStreaming(false);
      setStreamingText('');
      streamingTextRef.current = '';
    });

    return () => {
      unsubToken();
      unsubDone();
      unsubError();
    };
  }, []);

  useEffect(() => {
    if (!speaking) {
      return;
    }
    const interval = window.setInterval(() => {
      void LogosService.IsSpeaking()
        .then((active: unknown) => {
          if (!active) {
            stopReadingState();
          }
        })
        .catch(() => undefined);
    }, 450);
    return () => window.clearInterval(interval);
  }, [speaking]);

  useEffect(() => {
    return () => {
      sessionRef.current += 1;
      clearHighlightTimers();
    };
  }, []);

  useEffect(() => {
    setView('chat');
    setChatMessages([]);
    chatMessagesRef.current = [];
    setChatInput('');
    setStreaming(false);
    setStreamingText('');
    setSelectedOutput('');
    setSelectedEntry(null);
    setError(null);
    setIsSaved(false);
    setSavedToast(false);
    setSlashMenuOpen(false);
    currentActionRef.current = null;
    currentLabelRef.current = 'Logos Chat';
    streamingTextRef.current = '';
    void handleStop();
  }, [verseRef, chapterText]);

  useEffect(() => {
    if (!transcriptRef.current) {
      return;
    }
    transcriptRef.current.scrollTop = transcriptRef.current.scrollHeight;
  }, [chatMessages, streamingText, view]);

  async function beginStream(action: AIAction | 'chat', label: string, input: string) {
    setError(null);
    stopReadingState();
    setPrecaching(false);
    setSelectedOutput('');
    setIsSaved(false);
    setSavedToast(false);
    setStreaming(true);
    setStreamingText('');
    streamingTextRef.current = '';
    hasCommittedRef.current = false;
    setCurrentAction(action);
    setCurrentLabel(label);
    currentActionRef.current = action;
    currentLabelRef.current = label;

    await LogosService.StartAIStream(
      action === 'chat' ? 'ask' : action,
      verseRef,
      verseText,
      chapterText,
      bookName,
      chapterNum,
      translation,
      input,
    );
  }

  async function startQuickAction(action: AIAction) {
    setView('chat');
    setSelectedEntry(null);
    await beginStream(action, actionLabel[action], '');
  }

  async function sendChatMessage() {
    const question = chatInput.trim();
    if (!question || !aiAvailable || streaming) {
      return;
    }

    const nextUserMessage: ChatMessage = {
      id: createMessageId(),
      role: 'user',
      content: question,
      label: 'You',
      action: 'chat',
    };

    const nextConversation = [...chatMessagesRef.current, nextUserMessage];
    setChatMessages(nextConversation);
    chatMessagesRef.current = nextConversation;
    setChatInput('');
    await beginStream('chat', 'Logos Chat', buildConversationPrompt(nextConversation));
  }

  function stopStream() {
    void LogosService.StopAIStream();
    commitStreamingMessage('stopped');
  }

  async function handleReadAloud(text: string, entryKind?: string, entryId?: number) {
    if (speaking || starting) {
      const wasStarting = starting;
      stopReadingState();
      if (!wasStarting) {
        await LogosService.StopSpeaking().catch(() => undefined);
      }
      return;
    }

    const sessionId = sessionRef.current + 1;
    sessionRef.current = sessionId;
    clearHighlightTimers();
    setStarting(true);
    setActiveWord(-1);

    try {
      let plan: SyncedSpeechPlan;
      if (entryKind && entryId) {
        plan = (await LogosService.GetLibraryAudio(entryKind, entryId, text)) as SyncedSpeechPlan;
      } else {
        plan = (await LogosService.SpeakAIContent(text)) as SyncedSpeechPlan;
      }

      if (sessionRef.current !== sessionId) {
        return;
      }

      setSpeaking(true);
      setStarting(false);
      scheduleHighlights(plan, sessionId);
    } catch {
      if (sessionRef.current === sessionId) {
        stopReadingState();
      }
    }
  }

  async function handleStop() {
    stopReadingState();
    await LogosService.StopSpeaking().catch(() => undefined);
  }

  async function loadLibrary() {
    const entries = (await LogosService.ListLibrary()) as LibraryEntry[];
    setLibrary(entries ?? []);
    setSelectedEntry(null);
    setView('library');
  }

  function openEntry(entry: LibraryEntry) {
    setSelectedEntry(entry);
    void LogosService.StartAIPrecache(entry.content).catch(() => undefined);
  }

  async function autoSaveOutput(action: AIAction | 'chat' | null, finalText: string, ref: string) {
    setIsSaved(true);
    setSavedToast(true);
    if (savedToastTimerRef.current !== undefined) {
      window.clearTimeout(savedToastTimerRef.current);
    }
    savedToastTimerRef.current = window.setTimeout(() => setSavedToast(false), 2500);
    await LogosService.SaveToLibrary(
      outputKindForAction(action),
      outputTitleForAction(action, ref),
      ref,
      finalText,
      'ollama',
    );
  }

  async function saveResult() {
    if (!selectedOutput || isSaved) {
      return;
    }
    await autoSaveOutput(currentActionRef.current, selectedOutput, verseRef);
  }

  async function exportPDF(kind: string, title: string, ref: string, content: string) {
    await LogosService.ExportPDF(kind, title, ref, content);
  }

  function readButtonLabel() {
    if (starting) {
      return 'Starting...';
    }
    if (speaking) {
      return 'Stop';
    }
    if (precaching) {
      return 'Preparing...';
    }
    return 'Read Aloud';
  }

  function ReadAloudButton({ text, kind, id }: { text: string; kind?: string; id?: number }) {
    const isActive = speaking || starting;
    return (
      <button
        type="button"
        disabled={precaching && !isActive}
        onClick={() => void handleReadAloud(text, kind, id)}
        className={`flex-1 rounded-full border py-2 text-sm transition ${
          isActive
            ? 'border-red-500/40 bg-red-500/10 text-red-300 hover:bg-red-500/20'
            : 'border-gold/40 bg-gold/10 text-gold hover:bg-gold/20 disabled:opacity-50'
        }`}
      >
        {readButtonLabel()}
      </button>
    );
  }

  function handleComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (slashMenuOpen) {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        setSlashMenuIndex((i) => Math.min(i + 1, filteredSlashCommands.length - 1));
        return;
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault();
        setSlashMenuIndex((i) => Math.max(i - 1, 0));
        return;
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        event.preventDefault();
        const selected = filteredSlashCommands[slashMenuIndex];
        if (selected) void handleSlashCommand(selected.cmd);
        return;
      }
      if (event.key === 'Escape') {
        event.preventDefault();
        setSlashMenuOpen(false);
        return;
      }
    }
    if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault();
      void sendChatMessage();
    }
  }

  async function handleSlashCommand(cmd: SlashCmd) {
    setSlashMenuOpen(false);
    setChatInput('');
    switch (cmd) {
      case '/devotional':
      case '/sermon':
      case '/explain': {
        const actionMap: Record<string, AIAction> = {
          '/devotional': 'devotional',
          '/sermon': 'sermon',
          '/explain': 'explain_chapter',
        };
        const action = actionMap[cmd] as AIAction;
        const userMsg: ChatMessage = {
          id: createMessageId(),
          role: 'user',
          content: cmd,
          label: 'You',
          action: 'chat',
        };
        const nextConversation = [...chatMessagesRef.current, userMsg];
        setChatMessages(nextConversation);
        chatMessagesRef.current = nextConversation;
        await beginStream(action, actionLabel[action], '');
        break;
      }
      case '/save':
        await saveResult();
        break;
      case '/pdf':
        if (selectedOutput) {
          await exportPDF(
            outputKindForAction(currentActionRef.current),
            outputTitleForAction(currentActionRef.current, verseRef),
            verseRef,
            selectedOutput,
          );
        }
        break;
    }
  }

  const actionLabel: Record<AIAction, string> = {
    explain_verse: 'Explain Verse',
    explain_chapter: 'Pastoral Overview',
    devotional: 'Devotional',
    sermon: 'Sermon Outline',
    ask: 'Logos Chat',
  };
  const filteredSlashCommands = SLASH_COMMANDS.filter((c) =>
    c.cmd.startsWith(slashFilter),
  );
  const isThinking = streaming && streamingText.trim().length === 0;
  const showLibraryBack = mode === 'tools' && view === 'library';

  return (
    <div className="flex h-full min-h-0 flex-col rounded-[1.75rem] border border-border/80 bg-bg/60 backdrop-blur-xl">
      <div className="flex items-center justify-between border-b border-border/60 px-5 py-4">
        <div>
          <p className="text-xs uppercase tracking-[0.22em] text-muted">
            {mode === 'chat' ? 'Logos Chat' : 'AI Tools'}
          </p>
          <p className="mt-0.5 text-sm font-medium text-text">{verseRef || 'Open a chapter'}</p>
        </div>
        <div className="flex gap-2">
          {showLibraryBack && (
            <button
              type="button"
              onClick={() => {
                void handleStop();
                setSelectedEntry(null);
                setView('chat');
              }}
              className="rounded-full border border-border px-3 py-1 text-xs text-muted hover:text-text"
            >
              Back
            </button>
          )}
          <button
            type="button"
            onClick={() => {
              void handleStop();
              onClose();
            }}
            className="rounded-full border border-border px-3 py-1 text-xs text-muted hover:text-text"
          >
            Close
          </button>
        </div>
      </div>

      {mode === 'chat' && (
        <>
          <div className="border-b border-border/60 p-4">
            {!aiAvailable && (
              <div className="mb-4 rounded-[1.2rem] border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-xs text-yellow-200">
                Ollama is not running. Start Ollama to enable Logos Chat.
              </div>
            )}
            <div className="mb-4">
              <LocalSetupCard />
            </div>

            <div className="rounded-[1.35rem] border border-border bg-surface/55 p-4">
              <div className="flex flex-wrap items-center gap-2">
                <span className="rounded-full border border-gold/35 bg-gold/10 px-3 py-1 text-[0.65rem] uppercase tracking-[0.22em] text-gold">
                  {translation}
                </span>
                <span className="text-[0.72rem] uppercase tracking-[0.2em] text-muted">
                  {bookName} {chapterNum}
                </span>
              </div>
              <p className="mt-3 text-sm leading-7 text-muted">
                Ask about the passage open in the center reader. This pane stays focused on the conversation only.
              </p>
            </div>
          </div>

          <div ref={transcriptRef} className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-4">
            {chatMessages.length === 0 && !streaming && (
              <div className="rounded-[1.2rem] border border-dashed border-border/70 bg-bg/30 px-4 py-5 text-sm leading-7 text-muted">
                Start with a question like "What is happening in this passage?", "How should I apply this?", or
                "What would a pastor emphasize here?".
              </div>
            )}

            {chatMessages.map((message) => (
              <div key={message.id} className="rounded-[1.25rem] border border-border/70 bg-surface/40 px-4 py-3">
                <div className="mb-2 flex items-center gap-2">
                  <span
                    className={`rounded-full px-2.5 py-1 text-[0.68rem] font-semibold uppercase tracking-[0.18em] ${
                      message.role === 'user'
                        ? 'border border-accent/35 bg-accent/10 text-accent'
                        : 'border border-gold/35 bg-gold/10 text-gold'
                    }`}
                  >
                    {speakerName(message.role)}
                  </span>
                  {message.label && message.label !== 'You' && message.label !== 'Logos Chat' && (
                    <span className="text-[0.68rem] uppercase tracking-[0.18em] text-muted">{message.label}</span>
                  )}
                </div>
                <div
                  className="text-sm leading-7 text-text"
                  dangerouslySetInnerHTML={{ __html: `<p class="mb-3">${renderMarkdown(message.content)}</p>` }}
                />
              </div>
            ))}

            {streaming && (
              <div className="rounded-[1.25rem] border border-border/70 bg-surface/40 px-4 py-3">
                <div className="mb-2 flex items-center gap-2">
                  <span className="rounded-full border border-gold/35 bg-gold/10 px-2.5 py-1 text-[0.68rem] font-semibold uppercase tracking-[0.18em] text-gold">
                    Logos
                  </span>
                </div>
                {isThinking ? (
                  <div className="rounded-[1rem] border border-gold/20 bg-gold/5 px-4 py-3 text-sm text-gold">
                    <ThinkingSpinner />
                  </div>
                ) : (
                  <div
                    className="text-sm leading-7 text-text"
                    dangerouslySetInnerHTML={{
                      __html: `<p class="mb-3">${renderMarkdown(streamingText)}</p>`,
                    }}
                  />
                )}
              </div>
            )}
          </div>

          <div className="border-t border-border/60 bg-bg/65 p-4 backdrop-blur-xl">
            {error && <p className="mb-3 text-xs text-red-400">{error}</p>}
            {savedToast && (
              <div className="mb-3 flex items-center gap-2 rounded-full border border-green-500/30 bg-green-500/10 px-4 py-2 text-xs text-green-300">
                <svg className="h-3.5 w-3.5 shrink-0" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                  <path fillRule="evenodd" d="M16.707 5.293a1 1 0 00-1.414 0L8 12.586 4.707 9.293a1 1 0 00-1.414 1.414l4 4a1 1 0 001.414 0l8-8a1 1 0 000-1.414z" clipRule="evenodd" />
                </svg>
                Saved to library
              </div>
            )}
            <div className="relative rounded-[1.35rem] border border-border bg-bg/45 p-3">
              {slashMenuOpen && filteredSlashCommands.length > 0 && (
                <div className="absolute bottom-full left-0 right-0 mb-2 overflow-hidden rounded-[1.2rem] border border-border bg-bg/95 shadow-xl backdrop-blur-xl">
                  {filteredSlashCommands.map((item, i) => (
                    <button
                      key={item.cmd}
                      type="button"
                      onMouseDown={(e) => {
                        e.preventDefault();
                        void handleSlashCommand(item.cmd);
                      }}
                      className={`flex w-full items-center gap-3 px-4 py-2.5 text-left transition ${
                        i === slashMenuIndex ? 'bg-gold/15' : 'hover:bg-surface/60'
                      }`}
                    >
                      <span className="w-24 shrink-0 font-mono text-xs font-semibold text-gold/90">{item.cmd}</span>
                      <span className="text-xs text-muted">{item.description}</span>
                    </button>
                  ))}
                </div>
              )}
              <label className="mb-2 block text-[0.72rem] uppercase tracking-[0.18em] text-muted">
                Continue The Conversation
              </label>
              <textarea
                ref={composerRef}
                className="min-h-[94px] w-full resize-none rounded-[1rem] border border-border bg-surface/60 px-4 py-3 text-sm text-text placeholder:text-muted focus:border-gold/50 focus:outline-none"
                placeholder="Ask Logos about this passage, or type / for commands..."
                value={chatInput}
                onChange={(event) => {
                  const val = event.target.value;
                  setChatInput(val);
                  const slashMatch = val.match(/^(\/\S*)$/);
                  if (slashMatch) {
                    setSlashFilter(slashMatch[1].toLowerCase());
                    setSlashMenuIndex(0);
                    setSlashMenuOpen(true);
                  } else {
                    setSlashMenuOpen(false);
                  }
                }}
                onKeyDown={handleComposerKeyDown}
              />
              <div className="mt-3 flex items-center justify-between gap-3">
                <p className="text-[0.72rem] uppercase tracking-[0.18em] text-muted">
                  {isThinking ? (
                    <span className="inline-flex items-center gap-2 text-gold">
                      <ThinkingSpinner />
                    </span>
                  ) : streaming ? (
                    'Generating response...'
                  ) : (
                    'Enter to send • / for commands'
                  )}
                </p>
                {streaming ? (
                  <button
                    type="button"
                    onClick={stopStream}
                    className="rounded-full border border-red-500/40 bg-red-500/10 px-4 py-2 text-sm text-red-300 transition hover:bg-red-500/20"
                  >
                    Stop
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={() => void sendChatMessage()}
                    disabled={!chatInput.trim() || !aiAvailable || slashMenuOpen}
                    className="rounded-full border border-gold/40 bg-gold/10 px-4 py-2 text-sm text-gold transition hover:bg-gold/20 disabled:opacity-40"
                  >
                    Send
                  </button>
                )}
              </div>
            </div>
          </div>
        </>
      )}

      {mode === 'tools' && view !== 'library' && (
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {!aiAvailable && (
            <div className="mb-4 rounded-[1.2rem] border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-xs text-yellow-200">
              Ollama is not running. Start Ollama to enable AI tools.
            </div>
          )}
          <div className="mb-4">
            <LocalSetupCard />
          </div>

          <div className="rounded-[1.35rem] border border-border bg-surface/55 p-4">
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-full border border-accent/35 bg-accent/10 px-3 py-1 text-[0.65rem] uppercase tracking-[0.22em] text-accent">
                {translation}
              </span>
              <span className="text-[0.72rem] uppercase tracking-[0.2em] text-muted">
                {bookName} {chapterNum}
              </span>
            </div>
            <p className="mt-3 text-sm leading-7 text-muted">
              Generate focused study outputs for this passage without mixing them into the conversation transcript.
            </p>
            <div className="mt-4 flex flex-wrap gap-2">
              <button
                type="button"
                disabled={!aiAvailable || streaming}
                onClick={() => void startQuickAction('explain_chapter')}
                className="rounded-full border border-border bg-bg/40 px-3 py-2 text-xs text-text transition hover:border-gold/40 hover:text-gold disabled:opacity-40"
              >
                Explain this passage
              </button>
              <button
                type="button"
                disabled={!aiAvailable || streaming}
                onClick={() => void startQuickAction('devotional')}
                className="rounded-full border border-border bg-bg/40 px-3 py-2 text-xs text-text transition hover:border-gold/40 hover:text-gold disabled:opacity-40"
              >
                Write a devotional
              </button>
              <button
                type="button"
                disabled={!aiAvailable || streaming}
                onClick={() => void startQuickAction('sermon')}
                className="rounded-full border border-border bg-bg/40 px-3 py-2 text-xs text-text transition hover:border-gold/40 hover:text-gold disabled:opacity-40"
              >
                Build a sermon outline
              </button>
              <button
                type="button"
                onClick={() => void loadLibrary()}
                className="rounded-full border border-border/60 bg-bg/40 px-3 py-2 text-xs text-muted transition hover:text-text"
              >
                Library
              </button>
            </div>
          </div>

          {error && <p className="mt-4 text-xs text-red-400">{error}</p>}
          {savedToast && (
            <div className="mt-4 flex items-center gap-2 rounded-full border border-green-500/30 bg-green-500/10 px-4 py-2 text-xs text-green-300">
              <svg className="h-3.5 w-3.5 shrink-0" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                <path fillRule="evenodd" d="M16.707 5.293a1 1 0 00-1.414 0L8 12.586 4.707 9.293a1 1 0 00-1.414 1.414l4 4a1 1 0 001.414 0l8-8a1 1 0 000-1.414z" clipRule="evenodd" />
              </svg>
              Saved to library
            </div>
          )}

          <div className="mt-4 space-y-4">
            {!streaming && !selectedOutput && (
              <div className="rounded-[1.2rem] border border-dashed border-border/70 bg-bg/30 px-4 py-5 text-sm leading-7 text-muted">
                Choose one of the tools above to generate a passage explanation, devotional, or sermon outline.
              </div>
            )}

            {streaming && (
              <div className="rounded-[1.25rem] border border-border/70 bg-surface/40 px-4 py-3">
                <div className="mb-2 flex items-center gap-2">
                  <span className="rounded-full border border-accent/35 bg-accent/10 px-2.5 py-1 text-[0.68rem] font-semibold uppercase tracking-[0.18em] text-accent">
                    AI Tool
                  </span>
                  <span className="text-[0.68rem] uppercase tracking-[0.18em] text-muted">{currentLabel}</span>
                </div>
                {isThinking ? (
                  <div className="rounded-[1rem] border border-gold/20 bg-gold/5 px-4 py-3 text-sm text-gold">
                    <ThinkingSpinner />
                  </div>
                ) : (
                  <div
                    className="text-sm leading-7 text-text"
                    dangerouslySetInnerHTML={{
                      __html: `<p class="mb-3">${renderMarkdown(streamingText)}</p>`,
                    }}
                  />
                )}
                <div className="mt-4 flex justify-end">
                  <button
                    type="button"
                    onClick={stopStream}
                    className="rounded-full border border-red-500/40 bg-red-500/10 px-4 py-2 text-sm text-red-300 transition hover:bg-red-500/20"
                  >
                    Stop
                  </button>
                </div>
              </div>
            )}

            {!streaming && Boolean(selectedOutput) && (
              <div className="rounded-[1.25rem] border border-border/70 bg-surface/40 px-4 py-3">
                <div className="mb-2 flex items-center gap-2">
                  <span className="rounded-full border border-accent/35 bg-accent/10 px-2.5 py-1 text-[0.68rem] font-semibold uppercase tracking-[0.18em] text-accent">
                    AI Tool
                  </span>
                  <span className="text-[0.68rem] uppercase tracking-[0.18em] text-muted">{currentLabel}</span>
                </div>
                <div
                  className="text-sm leading-7 text-text"
                  dangerouslySetInnerHTML={{
                    __html: `<p class="mb-3">${renderMarkdown(selectedOutput)}</p>`,
                  }}
                />
                <div className="mt-4 flex flex-wrap gap-2">
                  <ReadAloudButton text={selectedOutput} />
                  <button
                    type="button"
                    onClick={() => void saveResult()}
                    disabled={isSaved}
                    className={`flex-1 rounded-full border py-2 text-sm transition ${
                      isSaved
                        ? 'cursor-default border-green-500/40 bg-green-500/10 text-green-300'
                        : 'border-accent/40 bg-accent/10 text-accent hover:bg-accent/20'
                    }`}
                  >
                    {isSaved ? 'Saved ✓' : 'Save'}
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      void exportPDF(
                        outputKindForAction(currentActionRef.current),
                        outputTitleForAction(currentActionRef.current, verseRef),
                        verseRef,
                        selectedOutput,
                      )
                    }
                    className="flex-1 rounded-full border border-border py-2 text-sm text-muted transition hover:text-text"
                  >
                    PDF
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {mode === 'tools' && view === 'library' && (
        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {!selectedEntry && (
            <div className="space-y-2">
            {library.length === 0 ? (
              <p className="text-sm text-muted">No saved items yet.</p>
            ) : (
              library.map((entry) => (
                <button
                  key={`${entry.kind}-${entry.id}`}
                  type="button"
                  onClick={() => openEntry(entry)}
                  className="w-full rounded-[1.35rem] border border-border bg-surface/60 px-4 py-3 text-left transition hover:border-gold/40"
                >
                  <p className="text-xs uppercase tracking-[0.18em] text-muted">
                    {entry.kind} - {entry.date}
                  </p>
                  <p className="mt-1 truncate text-sm text-text">{entry.title}</p>
                  {entry.ref && <p className="mt-0.5 text-xs text-muted">{entry.ref}</p>}
                </button>
              ))
            )}
            </div>
          )}

          {selectedEntry && (
            <div className="space-y-3">
              <p className="text-xs uppercase tracking-[0.2em] text-gold">{selectedEntry.kind}</p>
              <p className="text-sm text-muted">
                {selectedEntry.ref} - {selectedEntry.date}
              </p>

              {speaking ? (
                <div className="max-h-[50vh] overflow-y-auto rounded-[1.2rem] border border-border bg-surface/60 px-4 py-3">
                  <WordHighlight text={stripMarkdown(selectedEntry.content)} activeWord={activeWord} />
                </div>
              ) : (
                <div
                  className="max-h-[50vh] overflow-y-auto rounded-[1.2rem] border border-border bg-surface/60 px-4 py-3 text-sm leading-7 text-text"
                  dangerouslySetInnerHTML={{
                    __html: `<p class="mb-3">${renderMarkdown(selectedEntry.content)}</p>`,
                  }}
                />
              )}

              <div className="flex gap-2">
                <ReadAloudButton text={selectedEntry.content} kind={selectedEntry.kind} id={selectedEntry.id} />
                <button
                  type="button"
                  onClick={() =>
                    void exportPDF(selectedEntry.kind, selectedEntry.title, selectedEntry.ref, selectedEntry.content)
                  }
                  className="flex-1 rounded-full border border-border py-2 text-sm text-muted transition hover:text-text"
                >
                  PDF
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
