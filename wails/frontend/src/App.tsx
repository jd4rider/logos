import { useState, useEffect, useCallback } from 'react';
import { GetBibles, GetBooks, GetChapters, GetChapter } from './wailsjs/go/main/App';
import type { Bible, Book, Chapter, ChapterContent, SearchData } from './types';
import Sidebar from './components/Sidebar';
import Reader from './components/Reader';
import SearchPanel from './components/SearchPanel';
import TTSBar from './components/TTSBar';

type View = 'bibles' | 'books' | 'chapters' | 'reader' | 'search';

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

  return (
    <div className="flex flex-col h-screen bg-bg text-text">
      <header className="flex items-center justify-between px-4 py-2 bg-surface border-b border-border" style={{WebkitAppRegion: 'drag'} as React.CSSProperties}>
        <div className="flex items-center gap-3" style={{WebkitAppRegion: 'no-drag'} as React.CSSProperties}>
          <span className="text-gold font-bold text-xl">✝ Bible Reader</span>
          {currentBible && (
            <span className="text-accent text-sm">
              {currentBible.abbreviation}
              {currentBook && ` › ${currentBook.name}`}
              {currentChapter && ` › Ch.${currentChapter.number}`}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2" style={{WebkitAppRegion: 'no-drag'} as React.CSSProperties}>
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
              <Reader chapter={currentChapter} />
              <TTSBar
                chapter={currentChapter}
                onNext={currentChapter.next ? () => loadChapterById(currentChapter.next!.id) : undefined}
                onPrev={currentChapter.previous ? () => loadChapterById(currentChapter.previous!.id) : undefined}
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
      </div>
    </div>
  );
}
