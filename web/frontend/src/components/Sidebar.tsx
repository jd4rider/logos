import type { Bible, Book, Chapter } from '../types';

type View = 'bibles' | 'books' | 'chapters' | 'reader' | 'search';

interface Props {
  view: View;
  bibles: Bible[];
  books: Book[];
  chapters: Chapter[];
  loading: boolean;
  onSelectBible: (b: Bible) => void;
  onSelectBook: (b: Book) => void;
  onSelectChapter: (c: Chapter) => void;
}

export default function Sidebar({ view, bibles, books, chapters, loading, onSelectBible, onSelectBook, onSelectChapter }: Props) {
  const itemClass = "px-3 py-2 cursor-pointer hover:bg-highlight border-b border-border/50 transition-colors";
  const selectedClass = "bg-highlight border-l-2 border-l-gold";

  if (loading && !bibles.length && !books.length && !chapters.length) {
    return (
      <aside className="w-64 bg-surface border-r border-border flex items-center justify-center">
        <span className="text-gold animate-pulse">Loading…</span>
      </aside>
    );
  }

  return (
    <aside className="w-64 bg-surface border-r border-border overflow-y-auto flex-shrink-0">
      {view === 'bibles' && (
        <>
          <div className="px-3 py-2 text-xs text-muted uppercase tracking-wider border-b border-border">
            Translations
          </div>
          {bibles.map(b => (
            <div key={b.id} className={itemClass} onClick={() => onSelectBible(b)}>
              <div className="text-text text-sm font-medium">{b.name}</div>
              <div className="text-muted text-xs">{b.abbreviation} · {b.language.name}</div>
            </div>
          ))}
        </>
      )}
      {(view === 'books' || view === 'chapters' || view === 'reader' || view === 'search') && (
        <>
          <div className="px-3 py-2 text-xs text-muted uppercase tracking-wider border-b border-border">
            Books
          </div>
          {books.map(b => (
            <div key={b.id} className={`${itemClass} ${view !== 'bibles' && view !== 'books' ? '' : ''}`} onClick={() => onSelectBook(b)}>
              <div className="text-text text-sm">{b.name}</div>
            </div>
          ))}
        </>
      )}
      {view === 'chapters' && chapters.length > 0 && (
        <>
          <div className="px-3 py-2 text-xs text-muted uppercase tracking-wider border-b border-border mt-2">
            Chapters
          </div>
          <div className="grid grid-cols-5 gap-1 p-2">
            {chapters.map(ch => (
              <button
                key={ch.id}
                onClick={() => onSelectChapter(ch)}
                className="py-1 text-xs text-center rounded bg-highlight hover:bg-gold hover:text-bg text-text transition-colors"
              >
                {ch.number}
              </button>
            ))}
          </div>
        </>
      )}
      {(view === 'reader') && chapters.length > 0 && (
        <>
          <div className="px-3 py-2 text-xs text-muted uppercase tracking-wider border-b border-border mt-2">
            Chapters
          </div>
          <div className="grid grid-cols-5 gap-1 p-2">
            {chapters.map(ch => (
              <button
                key={ch.id}
                onClick={() => onSelectChapter(ch)}
                className="py-1 text-xs text-center rounded bg-highlight hover:bg-gold hover:text-bg text-text transition-colors"
              >
                {ch.number}
              </button>
            ))}
          </div>
        </>
      )}
    </aside>
  );
}
