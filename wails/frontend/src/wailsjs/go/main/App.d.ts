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
export function CancelImport(): Promise<void>;
export function ExportPDF(arg1:string,arg2:string,arg3:string,arg4:string):Promise<string>;
export function GetActiveVoice():Promise<main.VoiceEntry>;
export function GetTTSRate():Promise<number>;
export function ImportBibleFile(arg1:string,arg2:string,arg3:string,arg4:string):Promise<void>;
export function ImportBibleURL(arg1:string,arg2:string,arg3:string,arg4:string):Promise<void>;
export function IsAIAvailable():Promise<boolean>;
export function IsPaused():Promise<boolean>;
export function ListAIModels():Promise<Array<string>>;
export function ListLibrary():Promise<Array<main.LibraryEntry>>;
export function ListVoices():Promise<Array<main.VoiceEntry>>;
export function OpenFileDialog():Promise<string>;
export function PauseSpeaking():Promise<void>;
export function ResumeSpeaking():Promise<void>;
export function SaveToLibrary(arg1:string,arg2:string,arg3:string,arg4:string,arg5:string):Promise<void>;
export function SetTTSRate(arg1:number):Promise<void>;
export function SetVoice(arg1:string):Promise<void>;
export function SpeakSynced(arg1:string):Promise<Array<number>>;
export function StartAIStream(arg1:string,arg2:string,arg3:string,arg4:string,arg5:string,arg6:string,arg7:string,arg8:string):Promise<void>;
export function StopAIStream():Promise<void>;

export namespace main {
  export interface VoiceEntry {
    name: string;
    id: string;
    engine: string;
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
}
