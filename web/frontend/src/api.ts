import type { Bible, Book, Chapter, ChapterContent, SearchData } from './types';

const API_BASE = '/api';

async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export const api = {
  getBibles: (language = 'eng') =>
    fetchJSON<Bible[]>(`${API_BASE}/bibles?language=${language}`),

  getBooks: (bibleId: string) =>
    fetchJSON<Book[]>(`${API_BASE}/bibles/${bibleId}/books`),

  getChapters: (bibleId: string, bookId: string) =>
    fetchJSON<Chapter[]>(`${API_BASE}/bibles/${bibleId}/books/${bookId}/chapters`),

  getChapter: (bibleId: string, chapterId: string) =>
    fetchJSON<ChapterContent>(`${API_BASE}/bibles/${bibleId}/chapters/${chapterId}`),

  search: (bibleId: string, query: string, limit = 20) =>
    fetchJSON<SearchData>(
      `${API_BASE}/bibles/${bibleId}/search?query=${encodeURIComponent(query)}&limit=${limit}`
    ),

  getTTSEngine: () =>
    fetchJSON<{ engine: string; available: string }>(`${API_BASE}/tts/engine`),

  speak: (text: string) =>
    fetchJSON<{ ok: boolean }>(`${API_BASE}/tts/speak`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text }),
    }),

  stopSpeaking: () =>
    fetchJSON<{ ok: boolean }>(`${API_BASE}/tts/stop`, { method: 'POST' }),
};
