import { useState, useEffect, useCallback } from 'react';
import { GetBibles, GetBooks, GetChapters, GetChapter } from './wailsjs/go/main/App';
import type { Bible, Book, Chapter, ChapterContent, SearchData } from './types';
import Sidebar from './components/Sidebar';
import Reader from './components/Reader';
import SearchPanel from './components/SearchPanel';
import TTSBar from './components/TTSBar';
import AIPanel from './components/AIPanel';
import ImporterModal from './components/ImporterModal';

type View = 'bibles' | 'books' | 'chapters' | 'reader' | 'search';

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2 border-b border-border/60 last:border-b-0">
      <span className="text-xs uppercase tracking-wider text-muted">{label}</span>
      <span className="text-right text-sm text-text">{value}</span>
    </div>
  );
}

export default function App() {
  const [view, setView] = useState<View>('bibles');
  const [bibles, setBibles] = useState<Bible[]>([]);
  const [books, setBooks] = useState<Book[]>([]);
  const [chapters, setChapters] = useState<Chapter[]>([]);
  const [currentBible, setCurrentBible] = useState<Bible | null>(null);
  const [currentBook, setCurrentBook] = useState<Book | null>(null);
  const [currentChapter, setCurrentChapter] = useState<ChapterContent | null>(null);
  const [searchResults, setSearchResults] = useState<SearchData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSearch, setShowSearch] = useState(false);
  const [highlightWordIndex, setHighlightWordIndex] = useState(-1);
  const [showAIPanel, setShowAIPanel] = useState(false);
  const [showImporter, setShowImporter] = useState(false);

  useEffect(() => {
    setLoading(true);
    GetBibles('eng')
      .then(setBibles)
      .catch(e => setError(String(e)))
      .finally(() => setLoading(false));
  }, []);

  const selectBible = useCallback(async (bible: Bible) => {
    setCurrentBible(bible);
    setLoading(true);
    try {
      const b = await GetBooks(bible.id);
      setBooks(b);
      setView('books');
    } catch (e) { setError(String(e)); }
    finally { setLoading(false); }
  }, []);

  const selectBook = useCallback(async (book: Book) => {
    if (!currentBible) return;
    setCurrentBook(book);
    setLoading(true);
    try {
      const chs = await GetChapters(currentBible.id, book.id);
      setChapters(chs);
      setView('chapters');
    } catch (e) { setError(String(e)); }
    finally { setLoading(false); }
  }, [currentBible]);

  const selectChapter = useCallback(async (chapter: Chapter) => {
    if (!currentBible) return;
    setLoading(true);
    try {
      const ch = await GetChapter(currentBible.id, chapter.id);
      setCurrentChapter(ch);
      setView('reader');
    } catch (e) { setError(String(e)); }
    finally { setLoading(false); }
  }, [currentBible]);

  const loadChapterById = useCallback(async (chapterId: string) => {
    if (!currentBible) return;
    setLoading(true);
    setHighlightWordIndex(-1);
    try {
      const ch = await GetChapter(currentBible.id, chapterId);
      setCurrentChapter(ch);
      setView('reader');
    } catch (e) { setError(String(e)); }
    finally { setLoading(false); }
  }, [currentBible]);

  const goBack = () => {
    if (showSearch) { setShowSearch(false); return; }
    if (view === 'reader') setView('chapters');
    else if (view === 'chapters') setView('books');
    else if (view === 'books') setView('bibles');
  };

  const locationSummary = currentBible
    ? [
        currentBible.abbreviation,
        currentBook?.name,
        currentChapter ? `Chapter ${currentChapter.number}` : null,
      ].filter(Boolean).join(' / ')
    : 'No selection yet';

  // Build verse context for AI panel
  const verseRef = currentChapter
    ? `${currentBook?.name ?? ''} ${currentChapter.number}`
    : '';
  const verseText = '';

  return (
    <div className="flex flex-col h-screen bg-bg text-text">
      <header className="flex items-center justify-between px-4 py-2 bg-surface border-b border-border" style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}>
        <div className="flex items-center gap-3" style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}>
          <span className="text-gold font-bold text-xl">✝ Bible Reader</span>
          {currentBible && (
            <span className="text-accent text-sm">
              {currentBible.abbreviation}
              {currentBook && ` › ${currentBook.name}`}
              {currentChapter && ` › Ch.${currentChapter.number}`}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2" style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}>
          {view === 'reader' && (
            <button
              onClick={() => setShowAIPanel(!showAIPanel)}
              className={`px-3 py-1 text-sm border rounded transition-colors ${showAIPanel ? 'border-gold text-gold bg-highlight' : 'bg-highlight text-accent border-border hover:border-gold'}`}
            >
              ✝ AI
            </button>
          )}
          <button
            onClick={() => setShowImporter(true)}
            className="px-3 py-1 text-sm bg-highlight text-accent border border-border rounded hover:border-gold transition-colors"
          >
            📥 Import
          </button>
          {(view === 'reader' || view === 'books') && (
            <button
              onClick={() => setShowSearch(!showSearch)}
              className="px-3 py-1 text-sm bg-highlight text-accent border border-border rounded hover:border-gold transition-colors"
            >
              🔍 Search
            </button>
          )}
          {view !== 'bibles' && (
            <button
              onClick={goBack}
              className="px-3 py-1 text-sm text-muted border border-border rounded hover:text-text transition-colors"
            >
              ← Back
            </button>
          )}
        </div>
      </header>

      {error && (
        <div className="bg-red-900/50 border-b border-red-700 px-4 py-2 text-red-300 text-sm flex justify-between">
          <span>✗ {error}</span>
          <button onClick={() => setError(null)}>✕</button>
        </div>
      )}

      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          view={view}
          bibles={bibles}
          books={books}
          chapters={chapters}
          loading={loading}
          onSelectBible={selectBible}
          onSelectBook={selectBook}
          onSelectChapter={selectChapter}
        />

        <main className="flex-1 flex flex-col overflow-hidden">
          {showSearch ? (
            <SearchPanel
              results={searchResults}
              loading={loading}
              bibleId={currentBible?.id ?? ''}
              onResults={setSearchResults}
              onSelectVerse={(verseId) => {
                const parts = verseId.split('.');
                if (parts.length >= 2) {
                  loadChapterById(parts[0] + '.' + parts[1]);
                  setShowSearch(false);
                }
              }}
            />
          ) : view === 'reader' && currentChapter ? (
            <>
              <Reader chapter={currentChapter} highlightWordIndex={highlightWordIndex} />
              <TTSBar
                chapter={currentChapter}
                onNext={currentChapter.next ? () => loadChapterById(currentChapter.next!.id) : undefined}
                onPrev={currentChapter.previous ? () => loadChapterById(currentChapter.previous!.id) : undefined}
                onWordIndex={setHighlightWordIndex}
              />
            </>
          ) : view === 'bibles' && !loading ? (
            <div className="flex items-center justify-center h-full text-muted">
              <div className="text-center">
                <div className="text-6xl mb-4">✝</div>
                <div className="text-xl font-bold text-gold mb-2">Bible Reader</div>
                <div className="text-sm">Select a translation from the sidebar</div>
              </div>
            </div>
          ) : loading ? (
            <div className="flex items-center justify-center h-full">
              <span className="text-gold animate-pulse text-lg">Loading…</span>
            </div>
          ) : null}
        </main>

        {/* Right: AI panel or status panel */}
        {showAIPanel && currentChapter ? (
          <AIPanel
            verseRef={`${currentBook?.name ?? ''} ${currentChapter.number}`}
            verseText=""
            chapterText={currentChapter.content}
            bookName={currentBook?.name ?? ''}
            chapterNum={currentChapter.number}
            translation={currentBible?.abbreviation ?? ''}
            onClose={() => setShowAIPanel(false)}
          />
        ) : (
          <aside className="w-72 border-l border-border bg-surface/70 backdrop-blur-sm overflow-y-auto flex-shrink-0">
            <div className="p-4 space-y-4">
              <section className="bg-surface border border-border rounded-lg p-4">
                <div className="flex items-center justify-between gap-3 mb-3">
                  <h2 className="text-sm font-semibold text-gold uppercase tracking-wider">Status</h2>
                  <span className="text-[11px] uppercase tracking-wider text-muted">{loading ? 'Loading' : 'Ready'}</span>
                </div>
                <DetailRow label="View" value={showSearch ? 'Search' : view} />
                <DetailRow label="Location" value={locationSummary} />
                <DetailRow label="Translation" value={currentBible?.name ?? 'Choose a Bible'} />
              </section>
              {view === 'reader' && (
                <section className="bg-surface border border-border rounded-lg p-4">
                  <h2 className="text-sm font-semibold text-gold uppercase tracking-wider mb-3">AI Study</h2>
                  <button
                    onClick={() => setShowAIPanel(true)}
                    className="w-full py-2 text-sm border border-border rounded hover:border-gold hover:text-gold text-muted transition-colors"
                  >
                    ✦ Open AI Panel
                  </button>
                </section>
              )}
              <section className="bg-surface border border-border rounded-lg p-4">
                <h2 className="text-sm font-semibold text-gold uppercase tracking-wider mb-3">Import</h2>
                <button
                  onClick={() => setShowImporter(true)}
                  className="w-full py-2 text-sm border border-border rounded hover:border-gold hover:text-gold text-muted transition-colors"
                >
                  📥 Import Bible
                </button>
              </section>
            </div>
          </aside>
        )}
      </div>

      {showImporter && <ImporterModal onClose={() => setShowImporter(false)} />}
    </div>
  );
}
