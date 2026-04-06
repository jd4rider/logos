import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { LogosService } from './bindings';
import type { BibleSummary, Book, Chapter, ChapterContent, SearchData } from './types';
import Sidebar from './components/Sidebar';
import Reader from './components/Reader';
import SearchPanel from './components/SearchPanel';
import TTSBar from './components/TTSBar';
import AIPanel from './components/AIPanel';
import ImporterModal from './components/ImporterModal';
import LocalSetupCard from './components/LocalSetupCard';
import { languageLabel, languageOptions } from './lib/languages';

const dragRegion = { WebkitAppRegion: 'drag' } as unknown as CSSProperties;
const noDragRegion = { WebkitAppRegion: 'no-drag' } as unknown as CSSProperties;
const maxParallelColumns = 3;
const comparisonSlotCount = maxParallelColumns - 1;

function explainError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function sourceBadgeClass(source: string) {
  if (source === 'local') {
    return 'border-gold/40 bg-gold/10 text-gold';
  }
  return 'border-accent/30 bg-accent/10 text-accent';
}

function normalizeBookKey(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]/g, '');
}

function matchesBook(candidate: Book, target: Book) {
  const candidateKeys = [
    normalizeBookKey(candidate.abbreviation),
    normalizeBookKey(candidate.name),
    normalizeBookKey(candidate.nameLong),
  ];
  const targetKeys = [
    normalizeBookKey(target.abbreviation),
    normalizeBookKey(target.name),
    normalizeBookKey(target.nameLong),
  ];

  return targetKeys.some((key) => candidateKeys.includes(key));
}

function sameSelections(left: string[], right: string[]) {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function normalizeComparisonSelections(primaryBibleId: string | undefined, bibles: BibleSummary[], current: string[]) {
  const available = bibles.filter((bible) => bible.id !== primaryBibleId);
  const used = new Set<string>();

  return Array.from({ length: comparisonSlotCount }, (_, slot) => {
    const preserved = current[slot];
    if (preserved && available.some((bible) => bible.id === preserved) && !used.has(preserved)) {
      used.add(preserved);
      return preserved;
    }

    const fallback = available.find((bible) => !used.has(bible.id));
    if (!fallback) {
      return '';
    }

    used.add(fallback.id);
    return fallback.id;
  });
}

function comparisonGridClass(columnCount: number) {
  if (columnCount <= 1) {
    return '';
  }
  if (columnCount === 2) {
    return 'xl:grid-cols-2';
  }
  return 'xl:grid-cols-2 2xl:grid-cols-3';
}

export default function App() {
  const [assistantPane, setAssistantPane] = useState<'chat' | 'tools' | null>(null);
  const [bibles, setBibles] = useState<BibleSummary[]>([]);
  const [books, setBooks] = useState<Book[]>([]);
  const [chapters, setChapters] = useState<Chapter[]>([]);
  const [currentBible, setCurrentBible] = useState<BibleSummary | null>(null);
  const [currentBook, setCurrentBook] = useState<Book | null>(null);
  const [currentChapter, setCurrentChapter] = useState<ChapterContent | null>(null);
  const [searchResults, setSearchResults] = useState<SearchData | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [busyLabel, setBusyLabel] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [selectedLanguage, setSelectedLanguage] = useState('eng');
  const [activeWordIndex, setActiveWordIndex] = useState(-1);
  const [verseJumpTarget, setVerseJumpTarget] = useState<string | null>(null);
  const [verseJumpToken, setVerseJumpToken] = useState(0);
  const [parallelColumnCount, setParallelColumnCount] = useState(1);
  const [comparisonBibleIds, setComparisonBibleIds] = useState<string[]>(['', '']);
  const [comparisonChapters, setComparisonChapters] = useState<(ChapterContent | null)[]>([null, null]);
  const [comparisonBusy, setComparisonBusy] = useState<boolean[]>([false, false]);
  const [comparisonErrors, setComparisonErrors] = useState<(string | null)[]>([null, null]);
  const wordClickRef = useRef<((i: number) => void) | undefined>(undefined);
  const comparisonBooksRef = useRef<Record<string, Book[]>>({});
  const comparisonChaptersRef = useRef<Record<string, Chapter[]>>({});

  async function loadBibles(language = selectedLanguage) {
    setBusyLabel('Loading translations');
    try {
      const nextBibles = (await LogosService.GetBibles(language)) as BibleSummary[];
      setBibles(nextBibles);
      if (currentBible && !nextBibles.some((bible) => bible.id === currentBible.id)) {
        setCurrentBible(null);
        setCurrentBook(null);
        setCurrentChapter(null);
        setSearchResults(null);
        setSearchOpen(false);
        setBooks([]);
        setChapters([]);
        setComparisonChapters([null, null]);
        setComparisonBusy([false, false]);
        setComparisonErrors([null, null]);
      }
    } catch (loadError) {
      setError(explainError(loadError));
    } finally {
      setBusyLabel('');
    }
  }

  useEffect(() => {
    void loadBibles(selectedLanguage);
  }, [selectedLanguage]);

  useEffect(() => {
    const maxColumns = Math.min(maxParallelColumns, Math.max(1, bibles.length));
    setParallelColumnCount((current) => Math.min(current, maxColumns));
  }, [bibles.length]);

  useEffect(() => {
    setComparisonBibleIds((current) => {
      const next = normalizeComparisonSelections(currentBible?.id, bibles, current);
      return sameSelections(current, next) ? current : next;
    });
  }, [bibles, currentBible?.id]);

  async function selectBible(bible: BibleSummary) {
    setCurrentBible(bible);
    setCurrentBook(null);
    setCurrentChapter(null);
    setActiveWordIndex(-1);
    setVerseJumpTarget(null);
    setSearchResults(null);
    setSearchOpen(false);
    setBooks([]);
    setChapters([]);
    setComparisonChapters([null, null]);
    setComparisonBusy([false, false]);
    setComparisonErrors([null, null]);
    setBusyLabel(`Loading ${bible.abbreviation}`);

    try {
      const nextBooks = (await LogosService.GetBooks(bible.id)) as Book[];
      setBooks(nextBooks);
    } catch (loadError) {
      setError(explainError(loadError));
    } finally {
      setBusyLabel('');
    }
  }

  function handleLanguageChange(language: string) {
    setSelectedLanguage(language);
  }

  function handleParallelColumnChange(columnCount: number) {
    const maxColumns = Math.min(maxParallelColumns, Math.max(1, bibles.length));
    setParallelColumnCount(Math.max(1, Math.min(columnCount, maxColumns)));
  }

  function setComparisonBibleId(slot: number, bibleId: string) {
    setComparisonBibleIds((current) => current.map((value, index) => (index === slot ? bibleId : value)));
  }

  function comparisonOptions(slot: number) {
    const blocked = new Set<string>();
    if (currentBible?.id) {
      blocked.add(currentBible.id);
    }
    comparisonBibleIds.forEach((selectedId, index) => {
      if (index !== slot && selectedId) {
        blocked.add(selectedId);
      }
    });

    return bibles.filter((bible) => bible.id === comparisonBibleIds[slot] || !blocked.has(bible.id));
  }

  async function selectBook(book: Book) {
    if (!currentBible) {
      return;
    }

    setCurrentBook(book);
    setCurrentChapter(null);
    setActiveWordIndex(-1);
    setVerseJumpTarget(null);
    setSearchOpen(false);
    setComparisonChapters([null, null]);
    setComparisonBusy([false, false]);
    setComparisonErrors([null, null]);
    setBusyLabel(`Loading ${book.name}`);

    try {
      const nextChapters = (await LogosService.GetChapters(currentBible.id, book.id)) as Chapter[];
      setChapters(nextChapters);
    } catch (loadError) {
      setError(explainError(loadError));
    } finally {
      setBusyLabel('');
    }
  }

  async function loadChapter(chapterId: string) {
    if (!currentBible) {
      return;
    }

    setActiveWordIndex(-1);
    setVerseJumpTarget(null);
    setBusyLabel(`Opening ${chapterId}`);
    try {
      const content = (await LogosService.GetChapter(currentBible.id, chapterId)) as ChapterContent;
      setCurrentChapter(content);

      const selectedBook = books.find((book) => book.id === content.bookId);
      if (selectedBook) {
        setCurrentBook(selectedBook);
      }

      // Kick off background TTS synthesis so audio is ready when user hits Read.
      void LogosService.PrecacheChapter(content.content ?? '');
      setComparisonChapters([null, null]);
      setComparisonBusy([false, false]);
      setComparisonErrors([null, null]);
    } catch (loadError) {
      setError(explainError(loadError));
    } finally {
      setBusyLabel('');
    }
  }

  async function openSearchResult(chapterId: string, bookId: string) {
    const targetBook = books.find((book) => book.id === bookId);
    if (targetBook && currentBook?.id !== bookId) {
      await selectBook(targetBook);
    }

    await loadChapter(chapterId);
    setSearchOpen(false);
  }

  async function runSearch(query: string) {
    if (!currentBible) {
      return;
    }

    setBusyLabel(`Searching ${currentBible.abbreviation}`);
    try {
      const results = (await LogosService.Search(currentBible.id, query, 30)) as SearchData;
      setSearchResults(results);
      setSearchOpen(true);
    } catch (searchError) {
      setError(explainError(searchError));
    } finally {
      setBusyLabel('');
    }
  }

  function goBack() {
    if (searchOpen) {
      setSearchOpen(false);
      return;
    }
    if (currentChapter) {
      setCurrentChapter(null);
      setActiveWordIndex(-1);
      setVerseJumpTarget(null);
      setComparisonChapters([null, null]);
      setComparisonBusy([false, false]);
      setComparisonErrors([null, null]);
      return;
    }
    if (currentBook) {
      setCurrentBook(null);
      setCurrentChapter(null);
      setActiveWordIndex(-1);
      setVerseJumpTarget(null);
      setChapters([]);
      setComparisonChapters([null, null]);
      setComparisonBusy([false, false]);
      setComparisonErrors([null, null]);
      return;
    }
    if (currentBible) {
      setCurrentBible(null);
      setCurrentBook(null);
      setCurrentChapter(null);
      setActiveWordIndex(-1);
      setVerseJumpTarget(null);
      setSearchResults(null);
      setBooks([]);
      setChapters([]);
      setComparisonChapters([null, null]);
      setComparisonBusy([false, false]);
      setComparisonErrors([null, null]);
    }
  }

  useEffect(() => {
    const visibleSlots = Math.max(0, parallelColumnCount - 1);
    if (!currentBible || !currentBook || !currentChapter || visibleSlots === 0) {
      setComparisonBusy([false, false]);
      setComparisonChapters([null, null]);
      setComparisonErrors([null, null]);
      return;
    }

    let cancelled = false;
    const seen = new Set<string>();
    const referenceBook = currentBook;
    const referenceChapter = currentChapter;

    for (let slot = 0; slot < comparisonSlotCount; slot += 1) {
      if (slot >= visibleSlots) {
        setComparisonBusy((current) => current.map((value, index) => (index === slot ? false : value)));
        setComparisonChapters((current) => current.map((value, index) => (index === slot ? null : value)));
        setComparisonErrors((current) => current.map((value, index) => (index === slot ? null : value)));
        continue;
      }

      const bibleId = comparisonBibleIds[slot];
      if (!bibleId) {
        setComparisonBusy((current) => current.map((value, index) => (index === slot ? false : value)));
        setComparisonChapters((current) => current.map((value, index) => (index === slot ? null : value)));
        setComparisonErrors((current) =>
          current.map((value, index) => (index === slot ? 'Choose another translation to compare this chapter.' : value)),
        );
        continue;
      }

      if (bibleId === currentBible.id || seen.has(bibleId)) {
        setComparisonBusy((current) => current.map((value, index) => (index === slot ? false : value)));
        setComparisonChapters((current) => current.map((value, index) => (index === slot ? null : value)));
        setComparisonErrors((current) =>
          current.map((value, index) => (index === slot ? 'Choose a different comparison translation.' : value)),
        );
        continue;
      }

      seen.add(bibleId);
      setComparisonBusy((current) => current.map((value, index) => (index === slot ? true : value)));
      setComparisonErrors((current) => current.map((value, index) => (index === slot ? null : value)));

      void (async () => {
        try {
          let comparisonBooks = comparisonBooksRef.current[bibleId];
          if (!comparisonBooks) {
            comparisonBooks = (await LogosService.GetBooks(bibleId)) as Book[];
            comparisonBooksRef.current[bibleId] = comparisonBooks;
          }

          const matchingBook = comparisonBooks.find((candidate) => matchesBook(candidate, referenceBook));
          if (!matchingBook) {
            throw new Error(`Could not match ${referenceBook.name} in the comparison translation.`);
          }

          const chapterCacheKey = `${bibleId}:${matchingBook.id}`;
          let chapterList = comparisonChaptersRef.current[chapterCacheKey];
          if (!chapterList) {
            chapterList = (await LogosService.GetChapters(bibleId, matchingBook.id)) as Chapter[];
            comparisonChaptersRef.current[chapterCacheKey] = chapterList;
          }

          const matchingChapter = chapterList.find((chapter) => chapter.number === referenceChapter.number);
          if (!matchingChapter) {
            throw new Error(`Could not find chapter ${referenceChapter.number} in the comparison translation.`);
          }

          const nextChapter = (await LogosService.GetChapter(bibleId, matchingChapter.id)) as ChapterContent;
          if (!cancelled) {
            setComparisonChapters((current) => current.map((value, index) => (index === slot ? nextChapter : value)));
          }
        } catch (comparisonLoadError) {
          if (!cancelled) {
            setComparisonChapters((current) => current.map((value, index) => (index === slot ? null : value)));
            setComparisonErrors((current) =>
              current.map((value, index) => (index === slot ? explainError(comparisonLoadError) : value)),
            );
          }
        } finally {
          if (!cancelled) {
            setComparisonBusy((current) => current.map((value, index) => (index === slot ? false : value)));
          }
        }
      })();
    }

    return () => {
      cancelled = true;
    };
  }, [parallelColumnCount, comparisonBibleIds, currentBible?.id, currentBook?.id, currentChapter?.id]);

  function renderComparisonPane(slot: number) {
    if (comparisonBusy[slot]) {
      return (
        <div className="mx-auto flex h-full w-full items-center justify-center px-5 py-5">
          <div className="w-full max-w-3xl rounded-[2rem] border border-border/80 bg-surface/60 p-6 text-sm text-muted shadow-panel backdrop-blur-xl">
            Loading comparison translation...
          </div>
        </div>
      );
    }

    const chapter = comparisonChapters[slot];
    if (chapter) {
      const label = bibles.find((bible) => bible.id === comparisonBibleIds[slot])?.abbreviation ?? `Compare ${slot + 2}`;
      return <Reader chapter={chapter} activeWordIndex={-1} readerLabel={label} compact sharedScroll />;
    }

    return (
      <div className="mx-auto flex h-full w-full items-center justify-center px-5 py-5">
        <div className="w-full max-w-3xl rounded-[2rem] border border-border/80 bg-surface/60 p-6 text-sm text-muted shadow-panel backdrop-blur-xl">
          {comparisonErrors[slot] ?? 'Choose another translation to open it side by side.'}
        </div>
      </div>
    );
  }

  function renderMainPane() {
    if (searchOpen && currentBible) {
      return (
        <SearchPanel
          bibleLabel={`${currentBible.abbreviation} · ${currentBible.name}`}
          loading={Boolean(busyLabel)}
          results={searchResults}
          onClose={() => setSearchOpen(false)}
          onSearch={runSearch}
          onSelectChapter={openSearchResult}
        />
      );
    }

    if (currentChapter) {
      if (parallelColumnCount > 1) {
        return (
          <div className="h-full overflow-y-auto">
            <div className={`grid min-h-full items-start gap-0 ${comparisonGridClass(parallelColumnCount)}`}>
              <div className="min-h-0">
                <Reader
                  chapter={currentChapter}
                  activeWordIndex={activeWordIndex}
                  readerLabel={currentBible?.abbreviation ?? 'Primary'}
                  compact
                  sharedScroll
                  onVerseClick={(verseNumber) => {
                    setVerseJumpTarget(verseNumber);
                    setVerseJumpToken((value) => value + 1);
                  }}
                  onWordClick={(i) => wordClickRef.current?.(i)}
                />
              </div>

              {Array.from({ length: parallelColumnCount - 1 }, (_, slot) => (
                <div key={`comparison-pane-${slot}`} className="min-h-0">
                  {renderComparisonPane(slot)}
                </div>
              ))}
            </div>
          </div>
        );
      }

      return (
        <Reader
          chapter={currentChapter}
          activeWordIndex={activeWordIndex}
          readerLabel={currentBible?.abbreviation ?? 'Reader'}
          onVerseClick={(verseNumber) => {
            setVerseJumpTarget(verseNumber);
            setVerseJumpToken((value) => value + 1);
          }}
          onWordClick={(i) => wordClickRef.current?.(i)}
        />
      );
    }

    if (currentBook) {
      return (
        <div className="mx-auto flex h-full w-full max-w-4xl items-center justify-center px-7 py-8">
          <div className="w-full rounded-[2.3rem] border border-border/80 bg-surface/70 p-8 shadow-panel backdrop-blur-xl">
            <p className="mb-3 text-xs uppercase tracking-[0.24em] text-muted">Open Chapter</p>
            <h2 className="font-display text-4xl text-text">{currentBook.name}</h2>
            <p className="mt-4 max-w-2xl text-base leading-7 text-muted">
              Use the chapter section in the left stack to open the passage. The picker now collapses as you move from
              translation to book to chapter, so you should not have to scroll down through a long rail anymore.
            </p>
          </div>
        </div>
      );
    }

    if (currentBible) {
      return (
        <div className="mx-auto flex h-full w-full max-w-4xl items-center justify-center px-7 py-8">
          <div className="w-full rounded-[2.3rem] border border-border/80 bg-surface/70 p-8 shadow-panel backdrop-blur-xl">
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <span
                className={`rounded-full border px-3 py-1 text-[0.68rem] uppercase tracking-[0.22em] ${sourceBadgeClass(currentBible.source)}`}
              >
                {currentBible.source}
              </span>
              <span className="text-xs uppercase tracking-[0.24em] text-muted">
                {currentBible.language.name}
              </span>
            </div>
            <h2 className="font-display text-4xl text-text">{currentBible.name}</h2>
            <p className="mt-4 max-w-2xl text-base leading-7 text-muted">
              Browse books from the left rail or open search to jump directly to a passage. Local imports stay in the
              bundled SQLite library instead of hitting the API.
            </p>
          </div>
        </div>
      );
    }

    return (
      <div className="mx-auto flex h-full w-full max-w-5xl items-center justify-center px-7 py-8">
        <div className="grid w-full gap-5 lg:grid-cols-[1.4fr_1fr]">
          <section className="rounded-[2.5rem] border border-border/80 bg-surface/75 p-8 shadow-panel backdrop-blur-xl">
            <p className="mb-3 text-xs uppercase tracking-[0.28em] text-gold">Logos AI</p>
            <h1 className="max-w-xl font-display text-5xl leading-tight text-text">
              A tray-first scripture desk backed by the same Logos engine as the CLI.
            </h1>
            <p className="mt-5 max-w-2xl text-base leading-7 text-muted">
              Browse imported local translations, fall back to API.Bible when needed, search passages quickly, and read
              them aloud through the shared Logos speech stack with neural voices first and system fallbacks when
              needed.
            </p>
          </section>

          <section className="rounded-[2.5rem] border border-border/80 bg-bg/55 p-6 shadow-panel backdrop-blur-xl">
            <div className="space-y-4">
              <div className="rounded-[1.7rem] border border-border bg-surface/75 p-4">
                <p className="text-xs uppercase tracking-[0.22em] text-muted">Offline library</p>
                <p className="mt-2 text-sm leading-6 text-text">
                  Local translations are listed first and win abbreviation conflicts, so imported Bibles do not fall
                  through to API routes that would 403.
                </p>
              </div>
              <div className="rounded-[1.7rem] border border-border bg-surface/75 p-4">
                <p className="text-xs uppercase tracking-[0.22em] text-muted">Robust speech</p>
                <p className="mt-2 text-sm leading-6 text-text">
                  The speech card on the right uses the shared engine stack, including voice selection and rate
                  control, rather than browser speech synthesis.
                </p>
              </div>
              <div className="rounded-[1.7rem] border border-border bg-surface/75 p-4">
                <p className="text-xs uppercase tracking-[0.22em] text-muted">Menubar flow</p>
                <p className="mt-2 text-sm leading-6 text-text">
                  The Wails window is attached to the system tray and hides on focus loss or Escape so it behaves like a
                  proper menu bar utility.
                </p>
              </div>
            </div>
          </section>
        </div>
      </div>
    );
  }

  const maxColumnsAvailable = Math.min(maxParallelColumns, Math.max(1, bibles.length));

  return (
    <div className="flex h-screen flex-col text-text">
      <header
        className="relative z-10 border-b border-border/80 bg-bg/70 pl-20 pr-5 py-4 backdrop-blur-xl"
        style={dragRegion}
      >
        <div className="flex items-center justify-between gap-4">
          <div className="min-w-0" style={noDragRegion}>
            <div className="mb-1 flex flex-wrap items-center gap-3">
              <span className="rounded-full border border-gold/35 bg-gold/10 px-3 py-1 text-[0.68rem] uppercase tracking-[0.24em] text-gold">
                Menu Bar App
              </span>
              {currentBible && (
                <span
                  className={`rounded-full border px-3 py-1 text-[0.68rem] uppercase tracking-[0.22em] ${sourceBadgeClass(currentBible.source)}`}
                >
                  {currentBible.source}
                </span>
              )}
            </div>
            <div className="truncate font-display text-3xl text-text">
              Logos AI
              {currentBible && <span className="ml-3 text-xl text-muted">{currentBible.abbreviation}</span>}
            </div>
            <div className="mt-1 truncate text-sm text-muted">
              {currentBible?.name}
              {currentBook && ` / ${currentBook.name}`}
              {currentChapter && ` / ${currentChapter.reference}`}
            </div>
          </div>

          <div className="flex items-center gap-2" style={noDragRegion}>
            {currentChapter && (
              <button
                type="button"
                onClick={() => setAssistantPane((value) => (value === 'chat' ? null : 'chat'))}
                className={`rounded-full border px-4 py-2 text-sm transition ${
                  assistantPane === 'chat'
                    ? 'border-gold/60 bg-gold/15 text-gold'
                    : 'border-border bg-highlight/70 text-text hover:border-gold/50 hover:text-gold'
                }`}
              >
                Logos Chat
              </button>
            )}
            {currentChapter && (
              <button
                type="button"
                onClick={() => setAssistantPane((value) => (value === 'tools' ? null : 'tools'))}
                className={`rounded-full border px-4 py-2 text-sm transition ${
                  assistantPane === 'tools'
                    ? 'border-accent/60 bg-accent/15 text-accent'
                    : 'border-border bg-highlight/70 text-text hover:border-accent/50 hover:text-accent'
                }`}
              >
                AI Tools
              </button>
            )}
            <button
              type="button"
              onClick={() => setImportOpen(true)}
              className="rounded-full border border-border bg-highlight/70 px-4 py-2 text-sm text-text transition hover:border-accent/50 hover:text-accent"
            >
              Import
            </button>
            {currentBible && (
              <button
                type="button"
                onClick={() => setSearchOpen((value) => !value)}
                className="rounded-full border border-border bg-highlight/70 px-4 py-2 text-sm text-text transition hover:border-gold/50 hover:text-gold"
              >
                {searchOpen ? 'Close Search' : 'Search'}
              </button>
            )}
            {(currentBible || currentBook || currentChapter || searchOpen) && (
              <button
                type="button"
                onClick={goBack}
                className="rounded-full border border-border bg-bg/50 px-4 py-2 text-sm text-text transition hover:border-gold/50 hover:text-gold"
              >
                Back
              </button>
            )}
          </div>
        </div>
      </header>

      {error && (
        <div className="border-b border-red-500/30 bg-red-500/10 px-5 py-3 text-sm text-red-100">
          <div className="flex items-center justify-between gap-4">
            <span>{error}</span>
            <button type="button" onClick={() => setError(null)} className="text-xs uppercase tracking-[0.2em]">
              Dismiss
            </button>
          </div>
        </div>
      )}

      <div
        className={`grid min-h-0 flex-1 ${
          assistantPane === 'chat' && currentChapter
            ? 'lg:grid-cols-[320px_minmax(0,1fr)_420px]'
            : assistantPane === 'tools' && currentChapter
              ? 'lg:grid-cols-[320px_minmax(0,1fr)_380px]'
              : 'lg:grid-cols-[320px_minmax(0,1fr)_320px]'
        }`}
      >
        <Sidebar
          bibles={bibles}
          books={books}
          chapters={chapters}
          loading={Boolean(busyLabel)}
          selectedLanguage={selectedLanguage}
          languageOptions={languageOptions}
          currentBibleId={currentBible?.id}
          currentBookId={currentBook?.id}
          currentChapterId={currentChapter?.id}
          onLanguageChange={handleLanguageChange}
          onSelectBible={selectBible}
          onSelectBook={selectBook}
          onSelectChapter={(chapter) => void loadChapter(chapter.id)}
        />

        <main className="min-h-0 overflow-hidden">{renderMainPane()}</main>

        <aside
          className={`hidden min-h-0 border-l border-border/80 bg-surface/40 px-4 py-5 backdrop-blur-xl lg:block ${
            assistantPane && currentChapter ? 'overflow-hidden' : 'overflow-y-auto'
          }`}
        >
          <div className={assistantPane && currentChapter ? 'h-full' : 'space-y-5'}>
            {assistantPane && currentChapter ? (
              <AIPanel
                mode={assistantPane}
                verseRef={currentChapter.reference}
                verseText={''}
                chapterText={currentChapter.content}
                bookName={currentBook?.name ?? ''}
                chapterNum={currentChapter.number}
                translation={currentBible?.name ?? ''}
                onClose={() => setAssistantPane(null)}
              />
            ) : (
              <>
                <LocalSetupCard />
                {currentChapter && (
                  <section className="rounded-[1.75rem] border border-border/80 bg-bg/40 p-5 shadow-panel">
                    <div>
                      <p className="mb-2 text-xs uppercase tracking-[0.24em] text-muted">Compare</p>
                      <h3 className="font-display text-2xl text-text">Parallel Reading</h3>
                    </div>

                    <p className="mt-4 text-sm leading-7 text-muted">
                      Keep the same chapter synced across two or three translations so differences stay visible while
                      you read.
                    </p>

                    <div className="mt-4 grid grid-cols-3 gap-2">
                      {[1, 2, 3].map((columnCount) => (
                        <button
                          key={`parallel-columns-${columnCount}`}
                          type="button"
                          disabled={columnCount > maxColumnsAvailable}
                          onClick={() => handleParallelColumnChange(columnCount)}
                          className={`rounded-2xl border px-3 py-2 text-sm transition ${
                            parallelColumnCount === columnCount
                              ? 'border-gold bg-gold text-bg'
                              : 'border-border bg-highlight/70 text-text hover:border-gold/50 hover:text-gold disabled:cursor-not-allowed disabled:opacity-40'
                          }`}
                        >
                          {columnCount === 1 ? 'Single' : `${columnCount}-up`}
                        </button>
                      ))}
                    </div>

                    {maxColumnsAvailable === 1 ? (
                      <div className="mt-4 rounded-[1.2rem] border border-border bg-surface/60 px-4 py-3 text-sm text-muted">
                        Parallel reading needs at least two available translations in the current library.
                      </div>
                    ) : parallelColumnCount > 1 ? (
                      <div className="mt-4 space-y-3">
                        {Array.from({ length: parallelColumnCount - 1 }, (_, slot) => {
                          const options = comparisonOptions(slot);
                          return (
                            <label
                              key={`parallel-bible-${slot}`}
                              className="block text-xs uppercase tracking-[0.18em] text-muted"
                              htmlFor={`parallel-bible-${slot}`}
                            >
                              {slot === 0 ? 'Second translation' : 'Third translation'}
                              <select
                                id={`parallel-bible-${slot}`}
                                value={comparisonBibleIds[slot]}
                                onChange={(event) => setComparisonBibleId(slot, event.target.value)}
                                className="mt-2 w-full rounded-[1.1rem] border border-border bg-surface/70 px-4 py-3 text-sm normal-case text-text focus:border-gold/50 focus:outline-none"
                              >
                                {options.map((bible) => (
                                  <option key={bible.id} value={bible.id}>
                                    {bible.abbreviation} - {bible.name}
                                  </option>
                                ))}
                              </select>
                            </label>
                          );
                        })}
                      </div>
                    ) : (
                      <div className="mt-4 rounded-[1.2rem] border border-border bg-surface/60 px-4 py-3 text-sm text-muted">
                        Switch to 2-up or 3-up when you want to compare the same chapter side by side.
                      </div>
                    )}
                  </section>
                )}

                <section className="rounded-[1.75rem] border border-border/80 bg-bg/40 p-5 shadow-panel">
                  <p className="mb-3 text-xs uppercase tracking-[0.24em] text-muted">Status</p>
                  <div className="space-y-3">
                    <div className="rounded-[1.35rem] border border-border bg-surface/70 px-4 py-3">
                      <div className="text-xs uppercase tracking-[0.22em] text-muted">Selection</div>
                      <div className="mt-1 text-sm text-text">
                        {currentBible?.abbreviation ?? 'Choose a translation'}
                      </div>
                    </div>
                    <div className="rounded-[1.35rem] border border-border bg-surface/70 px-4 py-3">
                      <div className="text-xs uppercase tracking-[0.22em] text-muted">Activity</div>
                      <div className="mt-1 text-sm text-text">{busyLabel || 'Ready'}</div>
                    </div>
                    <div className="rounded-[1.35rem] border border-border bg-surface/70 px-4 py-3">
                      <div className="text-xs uppercase tracking-[0.22em] text-muted">Reader Layout</div>
                      <div className="mt-1 text-sm text-text">
                        {parallelColumnCount === 1 ? 'Single translation' : `${parallelColumnCount} synced columns`}
                      </div>
                    </div>
                    <div className="rounded-[1.35rem] border border-border bg-surface/70 px-4 py-3">
                      <div className="text-xs uppercase tracking-[0.22em] text-muted">Language Filter</div>
                      <div className="mt-1 text-sm text-text">{languageLabel(selectedLanguage)}</div>
                    </div>
                    {currentBible && (
                      <div className="rounded-[1.35rem] border border-border bg-surface/70 px-4 py-3">
                        <div className="text-xs uppercase tracking-[0.22em] text-muted">Translation</div>
                        <div className="mt-1 text-sm text-text">{currentBible.name}</div>
                        <div className="mt-2 text-xs uppercase tracking-[0.18em] text-muted">
                          {currentBible.language.name}
                        </div>
                      </div>
                    )}
                  </div>
                </section>

                {currentChapter ? (
                  <TTSBar
                    chapter={currentChapter}
                    activeWordIndex={activeWordIndex}
                    verseJumpTarget={verseJumpTarget}
                    verseJumpToken={verseJumpToken}
                    onError={setError}
                    onWordIndexChange={setActiveWordIndex}
                    onPrev={currentChapter.previous ? () => void loadChapter(currentChapter.previous!.id) : undefined}
                    onNext={currentChapter.next ? () => void loadChapter(currentChapter.next!.id) : undefined}
                    onRegisterWordClick={(fn) => { wordClickRef.current = fn; }}
                  />
                ) : (
                  <section className="rounded-[1.75rem] border border-border/80 bg-bg/40 p-5 shadow-panel">
                    <p className="mb-3 text-xs uppercase tracking-[0.24em] text-muted">Speech</p>
                    <h3 className="font-display text-2xl text-text">Ready</h3>
                    <p className="mt-4 text-sm leading-7 text-muted">
                      Open a chapter to read it aloud using the shared speech engine, with Kokoro and Piper first and
                      native system fallback voices after that.
                    </p>
                  </section>
                )}
              </>
            )}
          </div>
        </aside>
      </div>

      {importOpen && (
        <ImporterModal
          onClose={() => setImportOpen(false)}
          onImported={() => void loadBibles(selectedLanguage)}
        />
      )}
    </div>
  );
}
