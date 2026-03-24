import { useEffect, useState } from 'react';
import type { SearchData } from '../types';

interface Props {
  bibleLabel: string;
  loading: boolean;
  results: SearchData | null;
  onClose: () => void;
  onSearch: (query: string) => Promise<void> | void;
  onSelectChapter: (chapterId: string, bookId: string) => void;
}

export default function SearchPanel({
  bibleLabel,
  loading,
  results,
  onClose,
  onSearch,
  onSelectChapter,
}: Props) {
  const [query, setQuery] = useState(results?.query ?? '');

  useEffect(() => {
    setQuery(results?.query ?? '');
  }, [results?.query, bibleLabel]);

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!query.trim()) {
      return;
    }
    await onSearch(query.trim());
  }

  return (
    <section className="mx-auto flex h-full w-full max-w-4xl flex-col overflow-hidden px-7 py-8">
      <div className="mb-6 rounded-[2rem] border border-border/80 bg-surface/75 p-6 shadow-panel backdrop-blur-xl">
        <div className="mb-4 flex items-center justify-between gap-3">
          <div>
            <p className="text-xs uppercase tracking-[0.24em] text-muted">Search</p>
            <h2 className="font-display text-3xl text-text">{bibleLabel}</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-full border border-border bg-highlight/70 px-4 py-2 text-sm text-text transition hover:border-gold/60 hover:text-gold"
          >
            Close
          </button>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-3 md:flex-row">
          <input
            type="text"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search the scriptures"
            className="min-w-0 flex-1 rounded-2xl border border-border bg-bg/70 px-4 py-3 text-text outline-none transition focus:border-gold"
            autoFocus
          />
          <button
            type="submit"
            disabled={loading || !query.trim()}
            className="rounded-2xl border border-gold bg-gold px-5 py-3 text-sm font-semibold text-bg transition hover:bg-[#ffd06d] disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? 'Searching...' : 'Search'}
          </button>
        </form>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto rounded-[2rem] border border-border/80 bg-surface/55 p-4 shadow-panel backdrop-blur-xl">
        {!results && !loading && (
          <div className="flex h-full items-center justify-center text-center text-muted">
            Enter a phrase, theme, or verse keyword to search this translation.
          </div>
        )}

        {loading && (
          <div className="flex h-full items-center justify-center text-gold">
            Searching the library...
          </div>
        )}

        {results && !loading && (
          <div className="space-y-3">
            <div className="flex items-center justify-between gap-3 rounded-2xl border border-border bg-bg/45 px-4 py-3 text-sm text-muted">
              <span>
                {results.total} matches for <span className="text-text">"{results.query}"</span>
              </span>
              <span>{results.verseCount} verses</span>
            </div>

            {results.verses.length === 0 && (
              <div className="rounded-2xl border border-border bg-bg/45 px-4 py-6 text-center text-muted">
                No matches found for this translation.
              </div>
            )}

            {results.verses.map((verse) => (
              <button
                key={verse.id}
                type="button"
                onClick={() => onSelectChapter(verse.chapterId, verse.bookId)}
                className="block w-full rounded-[1.6rem] border border-border bg-bg/50 px-5 py-4 text-left transition hover:border-gold/65 hover:bg-highlight/65"
              >
                <div className="mb-2 font-sans text-xs uppercase tracking-[0.22em] text-accent">
                  {verse.reference}
                </div>
                <div className="font-serif text-[1rem] leading-7 text-text">{verse.text}</div>
              </button>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
