export interface Language {
  id: string;
  name: string;
  nameLocal: string;
  script: string;
  scriptDirection: string;
}

export interface Bible {
  id: string;
  abbreviation: string;
  name: string;
  nameLocal: string;
  description: string;
  language: Language;
  type: string;
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

export interface VerseContent {
  id: string;
  bookId: string;
  chapterId: string;
  bibleId: string;
  reference: string;
  content: string;
  copyright: string;
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
