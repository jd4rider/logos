import type { Bible, Book, Chapter, ChapterContent, VerseContent, SearchData } from '../../../types';

export declare function GetBibles(language: string): Promise<Bible[]>;
export declare function GetBooks(bibleID: string): Promise<Book[]>;
export declare function GetChapters(bibleID: string, bookID: string): Promise<Chapter[]>;
export declare function GetChapter(bibleID: string, chapterID: string): Promise<ChapterContent>;
export declare function GetVerse(bibleID: string, verseID: string): Promise<VerseContent>;
export declare function Search(bibleID: string, query: string, limit: number): Promise<SearchData>;
export declare function GetTTSEngine(): Promise<string>;
export declare function SpeakText(text: string): Promise<void>;
export declare function StopSpeaking(): Promise<void>;
export declare function IsSpeaking(): Promise<boolean>;
