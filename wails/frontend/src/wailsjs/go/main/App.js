'use strict';

export function GetBibles(language) {
  return window['go']['main']['App']['GetBibles'](language);
}
export function GetBooks(bibleID) {
  return window['go']['main']['App']['GetBooks'](bibleID);
}
export function GetChapters(bibleID, bookID) {
  return window['go']['main']['App']['GetChapters'](bibleID, bookID);
}
export function GetChapter(bibleID, chapterID) {
  return window['go']['main']['App']['GetChapter'](bibleID, chapterID);
}
export function GetVerse(bibleID, verseID) {
  return window['go']['main']['App']['GetVerse'](bibleID, verseID);
}
export function Search(bibleID, query, limit) {
  return window['go']['main']['App']['Search'](bibleID, query, limit);
}
export function GetTTSEngine() {
  return window['go']['main']['App']['GetTTSEngine']();
}
export function SpeakText(text) {
  return window['go']['main']['App']['SpeakText'](text);
}
export function StopSpeaking() {
  return window['go']['main']['App']['StopSpeaking']();
}
export function IsSpeaking() {
  return window['go']['main']['App']['IsSpeaking']();
}
