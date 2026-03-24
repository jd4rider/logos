import type { BibleSummary, Book, Chapter } from '../types';

interface Props {
  bibles: BibleSummary[];
  books: Book[];
  chapters: Chapter[];
  loading: boolean;
  currentBibleId?: string;
  currentBookId?: string;
  currentChapterId?: string;
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

export default function Sidebar({
  bibles,
  books,
  chapters,
  loading,
  currentBibleId,
  currentBookId,
  currentChapterId,
  onSelectBible,
  onSelectBook,
  onSelectChapter,
}: Props) {
  return (
    <aside className="min-h-0 overflow-y-auto border-r border-border/80 bg-surface/55 px-4 py-5 backdrop-blur-xl">
      <div className="space-y-5">
        <section className="rounded-[1.75rem] border border-border/70 bg-bg/35 p-3">
          <div className="mb-3 flex items-center justify-between gap-3 px-2">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-muted">Translations</p>
              <h2 className="font-display text-xl text-text">Library</h2>
            </div>
            {loading && (
              <span className="rounded-full border border-gold/30 bg-gold/10 px-3 py-1 text-[0.68rem] uppercase tracking-[0.2em] text-gold">
                Loading
              </span>
            )}
          </div>

          <div className="space-y-2">
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
                  <div className="mt-1 text-xs uppercase tracking-[0.18em] text-muted">
                    {bible.language.name}
                  </div>
                </button>
              );
            })}
          </div>
        </section>

        <section className="rounded-[1.75rem] border border-border/70 bg-bg/35 p-3">
          <div className="mb-3 px-2">
            <p className="text-xs uppercase tracking-[0.24em] text-muted">Books</p>
          </div>

          {books.length === 0 ? (
            <div className="rounded-[1.35rem] border border-dashed border-border/80 bg-bg/35 px-4 py-6 text-sm text-muted">
              Choose a translation to load its books.
            </div>
          ) : (
            <div className="space-y-2">
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
                    <div className="mt-1 text-[0.7rem] uppercase tracking-[0.2em] text-muted">
                      {book.abbreviation}
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </section>

        <section className="rounded-[1.75rem] border border-border/70 bg-bg/35 p-3">
          <div className="mb-3 px-2">
            <p className="text-xs uppercase tracking-[0.24em] text-muted">Chapters</p>
          </div>

          {chapters.length === 0 ? (
            <div className="rounded-[1.35rem] border border-dashed border-border/80 bg-bg/35 px-4 py-6 text-sm text-muted">
              Select a book to browse chapters.
            </div>
          ) : (
            <div className="grid grid-cols-4 gap-2">
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
          )}
        </section>
      </div>
    </aside>
  );
}
