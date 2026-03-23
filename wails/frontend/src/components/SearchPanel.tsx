import { useState } from 'react';
import { Search } from '../wailsjs/go/main/App';
import type { SearchData } from '../types';

interface Props {
  results: SearchData | null;
  loading: boolean;
  bibleId: string;
  onResults: (data: SearchData) => void;
  onSelectVerse: (verseId: string) => void;
}

export default function SearchPanel({ results, loading, bibleId, onResults, onSelectVerse }: Props) {
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim() || !bibleId) return;
    setSearching(true);
    try {
      const data = await Search(bibleId, query.trim(), 30);
      onResults(data);
    } catch (e) {
      console.error('Search failed:', e);
    } finally {
      setSearching(false);
    }
  };

  return (
    <div className="flex flex-col h-full">
      <div className="p-4 border-b border-border bg-surface">
        <form onSubmit={handleSubmit} className="flex gap-2">
          <input
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search the scriptures…"
            className="flex-1 px-3 py-2 bg-highlight border border-border rounded text-text placeholder-muted focus:outline-none focus:border-gold"
            autoFocus
          />
          <button
            type="submit"
            disabled={searching || loading}
            className="px-4 py-2 bg-gold text-bg font-bold rounded hover:bg-yellow-500 disabled:opacity-50"
          >
            {searching ? '…' : 'Search'}
          </button>
        </form>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        {results && (
          <>
            <div className="text-muted text-sm mb-3">{results.total} results for "{results.query}"</div>
            {results.verses.map(v => (
              <div
                key={v.id}
                className="mb-3 p-3 bg-surface border border-border rounded cursor-pointer hover:border-gold transition-colors"
                onClick={() => onSelectVerse(v.id)}
              >
                <div className="text-accent font-bold text-sm mb-1">{v.reference}</div>
                <div className="text-text text-sm leading-relaxed">{v.text}</div>
              </div>
            ))}
          </>
        )}
        {!results && !searching && (
          <div className="flex items-center justify-center h-full text-muted">Enter a search query</div>
        )}
      </div>
    </div>
  );
}
