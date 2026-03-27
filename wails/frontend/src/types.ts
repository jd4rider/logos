export interface Language {
  id: string;
  name: string;
  nameLocal: string;
  script: string;
  scriptDirection: string;
}

export interface BibleSummary {
  id: string;
  abbreviation: string;
  name: string;
  nameLocal: string;
  description: string;
  language: Language;
  type: string;
  source: string;
}

export interface Book {
  id: string;
  bibleId: string;
  abbreviation: string;
  name: string;
  nameLong: string;
}

export interface Chapter {
  id: string;
  bibleId: string;
  bookId: string;
  number: string;
  position: number;
}

export interface ChapterRef {
  id: string;
  number: string;
  bookId: string;
}

export interface ChapterContent {
  id: string;
  bibleId: string;
  bookId: string;
  number: string;
  reference: string;
  content: string;
  copyright: string;
  verseCount: number;
  next?: ChapterRef;
  previous?: ChapterRef;
}

export interface SearchVerse {
  id: string;
  orgId: string;
  bookId: string;
  bibleId: string;
  chapterId: string;
  reference: string;
  text: string;
}

export interface SearchData {
  query: string;
  limit: number;
  offset: number;
  total: number;
  verseCount: number;
  verses: SearchVerse[];
}

export interface VoiceOption {
  name: string;
  id: string;
  engine: string;
  label: string;
}

export interface SpeechSettings {
  available: boolean;
  engine: string;
  rate: number;
  activeVoice?: VoiceOption | null;
  voices: VoiceOption[];
}

export interface SyncedSpeechPlan {
  wordDurationsMs: number[];
}

export interface LibraryEntry {
  kind: string;
  id: number;
  title: string;
  ref: string;
  content: string;
  model: string;
  date: string;
}

export type AIAction = 'explain_verse' | 'explain_chapter' | 'devotional' | 'sermon' | 'ask';
