import { useEffect, useState } from 'react';
import type { BibleSummary, Book, Chapter } from '../types';
import type { LanguageOption } from '../lib/languages';

type Panel = 'bibles' | 'books' | 'chapters';

interface Props {
  bibles: BibleSummary[];
  books: Book[];
  chapters: Chapter[];
  loading: boolean;
  selectedLanguage: string;
  languageOptions: LanguageOption[];
  currentBibleId?: string;
  currentBookId?: string;
  currentChapterId?: string;
  onLanguageChange: (language: string) => void;
  onSelectBible: (bible: BibleSummary) => void;
  onSelectBook: (book: Book) => void;
  onSelectChapter: (chapter: Chapter) => void;
}

function sourceBadge(source: string) {
  if (source === 'local') {
    return 'border-gold/40 bg-gold/10 text-gold';
  }
  return 'border-accent/30 bg-accent/10 text-accent';
}

function togglePanel(current: Panel | null, panel: Panel) {
  if (current === panel) {
    return null;
  }
  return panel;
}

function previewChapters(chapters: Chapter[], currentChapterId?: string) {
  if (chapters.length <= 5) {
    return chapters;
  }

  const selectedIndex = currentChapterId ? chapters.findIndex((chapter) => chapter.id === currentChapterId) : -1;
  if (selectedIndex < 0) {
    return chapters.slice(0, 5);
  }

  const start = Math.max(0, Math.min(selectedIndex-2, chapters.length - 5));
  return chapters.slice(start, start + 5);
}

export default function Sidebar({
  bibles,
  books,
  chapters,
  loading,
  selectedLanguage,
  languageOptions,
  currentBibleId,
  currentBookId,
  currentChapterId,
  onLanguageChange,
  onSelectBible,
  onSelectBook,
  onSelectChapter,
}: Props) {
  const [expandedPanel, setExpandedPanel] = useState<Panel | null>('bibles');

  const currentBible = bibles.find((bible) => bible.id === currentBibleId);
  const currentBook = books.find((book) => book.id === currentBookId);
  const currentChapter = chapters.find((chapter) => chapter.id === currentChapterId);
  const localCount = bibles.filter((bible) => bible.source === 'local').length;
  const apiCount = bibles.length - localCount;
  const chapterPreview = previewChapters(chapters, currentChapterId);

  useEffect(() => {
    if (!currentBibleId) {
      setExpandedPanel('bibles');
      return;
    }
    if (!currentBookId) {
      setExpandedPanel('books');
      return;
    }
    if (!currentChapterId) {
      setExpandedPanel('chapters');
      return;
    }
    setExpandedPanel(null);
  }, [currentBibleId, currentBookId, currentChapterId]);

  return (
    <aside className="min-h-0 overflow-y-auto border-r border-border/80 bg-surface/55 px-4 py-5 backdrop-blur-xl">
      <div className="space-y-4">
        <section className="rounded-[1.75rem] border border-border/70 bg-bg/35 p-3">
          <button
            type="button"
            onClick={() => setExpandedPanel((current) => togglePanel(current, 'bibles'))}
            className="flex w-full items-center justify-between gap-3 rounded-[1.2rem] px-2 py-1 text-left"
          >
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Translations</p>
              <h2 className="font-display text-xl text-text">Library</h2>
            </div>
            <div className="text-right">
              {loading && expandedPanel === 'bibles' ? (
                <span className="rounded-full border border-gold/30 bg-gold/10 px-3 py-1 text-[0.68rem] uppercase tracking-[0.2em] text-gold">
                  Loading
                </span>
              ) : (
                <span className="text-xs uppercase tracking-[0.22em] text-muted">{expandedPanel === 'bibles' ? 'Open' : 'Collapsed'}</span>
              )}
            </div>
          </button>

          {expandedPanel === 'bibles' || !currentBible ? (
            <div className="mt-3 space-y-2">
              <label className="block px-2">
                <span className="text-[0.68rem] uppercase tracking-[0.22em] text-muted">Language</span>
                <select
                  value={selectedLanguage}
                  disabled={loading}
                  onChange={(event) => onLanguageChange(event.target.value)}
                  className="mt-2 w-full rounded-[1rem] border border-border bg-bg/55 px-3 py-2 text-sm text-text outline-none transition focus:border-gold/45 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {languageOptions.map((option) => (
                    <option key={option.code || 'all'} value={option.code}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <div className="flex flex-wrap gap-2 px-2">
                <span className="rounded-full border border-gold/30 bg-gold/10 px-3 py-1 text-[0.62rem] uppercase tracking-[0.2em] text-gold">
                  {localCount} local
                </span>
                <span className="rounded-full border border-accent/30 bg-accent/10 px-3 py-1 text-[0.62rem] uppercase tracking-[0.2em] text-accent">
                  {apiCount} api
                </span>
              </div>

              {bibles.map((bible) => {
                const selected = currentBibleId === bible.id;
                return (
                  <button
                    key={bible.id}
                    type="button"
                    onClick={() => onSelectBible(bible)}
                    className={`block w-full rounded-[1.35rem] border px-4 py-3 text-left transition ${
                      selected
                        ? 'border-gold/70 bg-highlight/85 shadow-panel'
                        : 'border-border bg-bg/45 hover:border-gold/40 hover:bg-highlight/60'
                    }`}
                  >
                    <div className="mb-1 flex items-center justify-between gap-2">
                      <span className="font-display text-lg text-text">{bible.abbreviation}</span>
                      <span
                        className={`rounded-full border px-2 py-1 text-[0.62rem] uppercase tracking-[0.2em] ${sourceBadge(bible.source)}`}
                      >
                        {bible.source}
                      </span>
                    </div>
                    <div className="text-sm text-text">{bible.name}</div>
                    <div className="mt-1 text-xs uppercase tracking-[0.18em] text-muted">{bible.language.name}</div>
                  </button>
                );
              })}
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setExpandedPanel('bibles')}
              className="mt-3 block w-full rounded-[1.35rem] border border-gold/50 bg-highlight/75 px-4 py-3 text-left transition hover:border-gold"
            >
              <div className="mb-1 flex items-center justify-between gap-2">
                <span className="font-display text-lg text-text">{currentBible.abbreviation}</span>
                <span
                  className={`rounded-full border px-2 py-1 text-[0.62rem] uppercase tracking-[0.2em] ${sourceBadge(currentBible.source)}`}
                >
                  {currentBible.source}
                </span>
              </div>
              <div className="text-sm text-text">{currentBible.name}</div>
              <div className="mt-1 text-xs uppercase tracking-[0.18em] text-muted">{currentBible.language.name}</div>
            </button>
          )}
        </section>

        <section className="rounded-[1.75rem] border border-border/70 bg-bg/35 p-3">
          <button
            type="button"
            onClick={() => currentBible && setExpandedPanel((current) => togglePanel(current, 'books'))}
            disabled={!currentBible}
            className="flex w-full items-center justify-between gap-3 rounded-[1.2rem] px-2 py-1 text-left disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Books</p>
              <h2 className="font-display text-xl text-text">{currentBible ? 'Selection' : 'Waiting'}</h2>
            </div>
            <span className="text-xs uppercase tracking-[0.22em] text-muted">
              {expandedPanel === 'books' ? 'Open' : currentBook ? 'Collapsed' : ''}
            </span>
          </button>

          {!currentBible ? (
            <div className="mt-3 rounded-[1.35rem] border border-dashed border-border/80 bg-bg/35 px-4 py-6 text-sm text-muted">
              Choose a translation to load its books.
            </div>
          ) : expandedPanel === 'books' || !currentBook ? (
            <div className="mt-3 space-y-2">
              {books.map((book) => {
                const selected = currentBookId === book.id;
                return (
                  <button
                    key={book.id}
                    type="button"
                    onClick={() => onSelectBook(book)}
                    className={`block w-full rounded-[1.2rem] border px-4 py-3 text-left transition ${
                      selected
                        ? 'border-gold/65 bg-highlight/75'
                        : 'border-border bg-bg/40 hover:border-gold/35 hover:bg-highlight/55'
                    }`}
                  >
                    <div className="text-sm font-medium text-text">{book.name}</div>
                    <div className="mt-1 text-[0.7rem] uppercase tracking-[0.2em] text-muted">{book.abbreviation}</div>
                  </button>
                );
              })}
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setExpandedPanel('books')}
              className="mt-3 block w-full rounded-[1.2rem] border border-gold/45 bg-highlight/60 px-4 py-3 text-left transition hover:border-gold"
            >
              <div className="text-sm font-medium text-text">{currentBook.name}</div>
              <div className="mt-1 text-[0.7rem] uppercase tracking-[0.2em] text-muted">{currentBook.abbreviation}</div>
            </button>
          )}
        </section>

        <section className="rounded-[1.75rem] border border-border/70 bg-bg/35 p-3">
          <button
            type="button"
            onClick={() => currentBook && setExpandedPanel((current) => togglePanel(current, 'chapters'))}
            disabled={!currentBook}
            className="flex w-full items-center justify-between gap-3 rounded-[1.2rem] px-2 py-1 text-left disabled:cursor-not-allowed disabled:opacity-50"
          >
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Chapters</p>
              <h2 className="font-display text-xl text-text">{currentBook ? `${chapters.length} loaded` : 'Waiting'}</h2>
            </div>
            <span className="text-xs uppercase tracking-[0.22em] text-muted">
              {expandedPanel === 'chapters' ? 'Open' : currentChapter ? 'Collapsed' : ''}
            </span>
          </button>

          {!currentBook ? (
            <div className="mt-3 rounded-[1.35rem] border border-dashed border-border/80 bg-bg/35 px-4 py-6 text-sm text-muted">
              Select a book to load its chapters.
            </div>
          ) : expandedPanel === 'chapters' || !currentChapter ? (
            <div className="mt-3 grid grid-cols-5 gap-2">
              {chapters.map((chapter) => {
                const selected = currentChapterId === chapter.id;
                return (
                  <button
                    key={chapter.id}
                    type="button"
                    onClick={() => onSelectChapter(chapter)}
                    className={`rounded-2xl border px-0 py-2 text-center text-sm transition ${
                      selected
                        ? 'border-gold bg-gold text-bg'
                        : 'border-border bg-bg/45 text-text hover:border-gold/45 hover:bg-highlight/55'
                    }`}
                  >
                    {chapter.number}
                  </button>
                );
              })}
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setExpandedPanel('chapters')}
              className="mt-3 block w-full rounded-[1.2rem] border border-gold/45 bg-highlight/55 px-4 py-3 text-left transition hover:border-gold"
            >
              <div className="mb-2 flex items-center justify-between gap-3">
                <span className="text-sm font-medium text-text">Chapter {currentChapter.number}</span>
                <span className="text-[0.7rem] uppercase tracking-[0.2em] text-muted">{chapters.length} total</span>
              </div>
              <div className="grid grid-cols-5 gap-2">
                {chapterPreview.map((chapter) => {
                  const selected = currentChapterId === chapter.id;
                  return (
                    <span
                      key={chapter.id}
                      className={`rounded-xl border px-0 py-1.5 text-center text-xs ${
                        selected
                          ? 'border-gold bg-gold text-bg'
                          : 'border-border bg-bg/45 text-text'
                      }`}
                    >
                      {chapter.number}
                    </span>
                  );
                })}
              </div>
            </button>
          )}
        </section>
      </div>
    </aside>
  );
}
