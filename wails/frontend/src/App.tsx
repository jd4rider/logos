import { useEffect, useState, type CSSProperties } from 'react';
import { LogosService } from './bindings';
import type { BibleSummary, Book, Chapter, ChapterContent, SearchData } from './types';
import Sidebar from './components/Sidebar';
import Reader from './components/Reader';
import SearchPanel from './components/SearchPanel';
import TTSBar from './components/TTSBar';

const dragRegion = { WebkitAppRegion: 'drag' } as unknown as CSSProperties;
const noDragRegion = { WebkitAppRegion: 'no-drag' } as unknown as CSSProperties;

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

export default function App() {
  const [bibles, setBibles] = useState<BibleSummary[]>([]);
  const [books, setBooks] = useState<Book[]>([]);
  const [chapters, setChapters] = useState<Chapter[]>([]);
  const [currentBible, setCurrentBible] = useState<BibleSummary | null>(null);
  const [currentBook, setCurrentBook] = useState<Book | null>(null);
  const [currentChapter, setCurrentChapter] = useState<ChapterContent | null>(null);
  const [searchResults, setSearchResults] = useState<SearchData | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [busyLabel, setBusyLabel] = useState('');
  const [error, setError] = useState<string | null>(null);

  async function loadBibles() {
    setBusyLabel('Loading translations');
    try {
      const nextBibles = (await LogosService.GetBibles('eng')) as BibleSummary[];
      setBibles(nextBibles);
    } catch (loadError) {
      setError(explainError(loadError));
    } finally {
      setBusyLabel('');
    }
  }

  useEffect(() => {
    void loadBibles();
  }, []);

  async function selectBible(bible: BibleSummary) {
    setCurrentBible(bible);
    setCurrentBook(null);
    setCurrentChapter(null);
    setSearchResults(null);
    setSearchOpen(false);
    setBooks([]);
    setChapters([]);
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

  async function selectBook(book: Book) {
    if (!currentBible) {
      return;
    }

    setCurrentBook(book);
    setCurrentChapter(null);
    setSearchOpen(false);
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

    setBusyLabel(`Opening ${chapterId}`);
    try {
      const content = (await LogosService.GetChapter(currentBible.id, chapterId)) as ChapterContent;
      setCurrentChapter(content);

      const selectedBook = books.find((book) => book.id === content.bookId);
      if (selectedBook) {
        setCurrentBook(selectedBook);
      }
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
      return;
    }
    if (currentBook) {
      setCurrentBook(null);
      setCurrentChapter(null);
      setChapters([]);
      return;
    }
    if (currentBible) {
      setCurrentBible(null);
      setCurrentBook(null);
      setCurrentChapter(null);
      setSearchResults(null);
      setBooks([]);
      setChapters([]);
    }
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
      return <Reader chapter={currentChapter} />;
    }

    if (currentBook) {
      return (
        <div className="mx-auto flex h-full w-full max-w-4xl items-center justify-center px-7 py-8">
          <div className="w-full rounded-[2.3rem] border border-border/80 bg-surface/70 p-8 shadow-panel backdrop-blur-xl">
            <p className="mb-3 text-xs uppercase tracking-[0.24em] text-muted">Open Chapter</p>
            <h2 className="font-display text-4xl text-text">{currentBook.name}</h2>
            <p className="mt-4 max-w-xl text-base leading-7 text-muted">
              {chapters.length} chapters are loaded in the left rail. Choose one to open the passage in the reader.
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
              them aloud through the shared Piper or Kokoro pipeline.
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

  return (
    <div className="flex h-screen flex-col text-text">
      <header
        className="relative z-10 border-b border-border/80 bg-bg/70 px-5 py-4 backdrop-blur-xl"
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

      <div className="grid min-h-0 flex-1 lg:grid-cols-[320px_minmax(0,1fr)_320px]">
        <Sidebar
          bibles={bibles}
          books={books}
          chapters={chapters}
          loading={Boolean(busyLabel)}
          currentBibleId={currentBible?.id}
          currentBookId={currentBook?.id}
          currentChapterId={currentChapter?.id}
          onSelectBible={selectBible}
          onSelectBook={selectBook}
          onSelectChapter={(chapter) => void loadChapter(chapter.id)}
        />

        <main className="min-h-0 overflow-hidden">{renderMainPane()}</main>

        <aside className="hidden min-h-0 overflow-y-auto border-l border-border/80 bg-surface/40 px-4 py-5 backdrop-blur-xl lg:block">
          <div className="space-y-5">
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
                onError={setError}
                onPrev={currentChapter.previous ? () => void loadChapter(currentChapter.previous!.id) : undefined}
                onNext={currentChapter.next ? () => void loadChapter(currentChapter.next!.id) : undefined}
              />
            ) : (
              <section className="rounded-[1.75rem] border border-border/80 bg-bg/40 p-5 shadow-panel">
                <p className="mb-3 text-xs uppercase tracking-[0.24em] text-muted">Speech</p>
                <h3 className="font-display text-2xl text-text">Ready</h3>
                <p className="mt-4 text-sm leading-7 text-muted">
                  Open a chapter to read it aloud using the shared Piper or Kokoro speech engine.
                </p>
              </section>
            )}
          </div>
        </aside>
      </div>
    </div>
  );
}
