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
export function CancelImport() {
  return window['go']['main']['App']['CancelImport']();
}
export function ExportPDF(arg1, arg2, arg3, arg4) {
  return window['go']['main']['App']['ExportPDF'](arg1, arg2, arg3, arg4);
}
export function GetActiveVoice() {
  return window['go']['main']['App']['GetActiveVoice']();
}
export function GetTTSRate() {
  return window['go']['main']['App']['GetTTSRate']();
}
export function ImportBibleFile(arg1, arg2, arg3, arg4) {
  return window['go']['main']['App']['ImportBibleFile'](arg1, arg2, arg3, arg4);
}
export function ImportBibleURL(arg1, arg2, arg3, arg4) {
  return window['go']['main']['App']['ImportBibleURL'](arg1, arg2, arg3, arg4);
}
export function IsAIAvailable() {
  return window['go']['main']['App']['IsAIAvailable']();
}
export function IsPaused() {
  return window['go']['main']['App']['IsPaused']();
}
export function ListAIModels() {
  return window['go']['main']['App']['ListAIModels']();
}
export function ListLibrary() {
  return window['go']['main']['App']['ListLibrary']();
}
export function ListVoices() {
  return window['go']['main']['App']['ListVoices']();
}
export function OpenFileDialog() {
  return window['go']['main']['App']['OpenFileDialog']();
}
export function PauseSpeaking() {
  return window['go']['main']['App']['PauseSpeaking']();
}
export function ResumeSpeaking() {
  return window['go']['main']['App']['ResumeSpeaking']();
}
export function SaveToLibrary(arg1, arg2, arg3, arg4, arg5) {
  return window['go']['main']['App']['SaveToLibrary'](arg1, arg2, arg3, arg4, arg5);
}
export function SetTTSRate(arg1) {
  return window['go']['main']['App']['SetTTSRate'](arg1);
}
export function SetVoice(arg1) {
  return window['go']['main']['App']['SetVoice'](arg1);
}
export function SpeakSynced(arg1) {
  return window['go']['main']['App']['SpeakSynced'](arg1);
}
export function StartAIStream(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8) {
  return window['go']['main']['App']['StartAIStream'](arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8);
}
export function StopAIStream() {
  return window['go']['main']['App']['StopAIStream']();
}
