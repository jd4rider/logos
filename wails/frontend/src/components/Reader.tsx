import type { ReactNode } from 'react';
import type { ChapterContent } from '../types';

interface Props {
  chapter: ChapterContent;
  activeWordIndex: number;
  readerLabel?: string;
  compact?: boolean;
  sharedScroll?: boolean;
  onVerseClick?: (verseNumber: string) => void;
  onWordClick?: (wordIndex: number) => void;
}

function renderWordSegment(
  segment: string,
  verseNumber: string,
  paragraphIndex: number,
  wordCursor: { value: number },
  activeWordIndex: number,
  onVerseClick?: (verseNumber: string) => void,
  onWordClick?: (wordIndex: number) => void,
): ReactNode[] {
  const words = segment.trim().split(/\s+/).filter(Boolean);
  const parts: ReactNode[] = [];

  words.forEach((word, wordIndex) => {
    if (wordIndex > 0) {
      parts.push(' ');
    }

    const currentIndex = wordCursor.value;
    const highlight = currentIndex === activeWordIndex;
    parts.push(
      <span
        key={`word-${paragraphIndex}-${currentIndex}`}
        className={
          highlight
            ? 'cursor-pointer rounded-md bg-gold/20 px-1 py-0.5 text-gold shadow-[0_0_0_1px_rgba(245,191,82,0.35)]'
            : 'cursor-pointer text-text transition hover:text-gold'
        }
        onClick={() => {
          if (onWordClick) {
            onWordClick(currentIndex);
          } else if (verseNumber) {
            onVerseClick?.(verseNumber);
          }
        }}
      >
        {word}
      </span>,
    );
    wordCursor.value += 1;
  });

  return parts;
}

function parseContent(
  content: string,
  activeWordIndex: number,
  onVerseClick?: (verseNumber: string) => void,
  onWordClick?: (wordIndex: number) => void,
): ReactNode[] {
  const cleaned = content.replace(/¶/g, '').trim();
  const paragraphs = cleaned.split(/\n+/);
  const wordCursor = { value: 0 };

  return paragraphs.map((paragraph, paragraphIndex) => {
    if (!paragraph.trim()) {
      return <br key={`break-${paragraphIndex}`} />;
    }

    const parts: ReactNode[] = [];
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    const verseNumberPattern = /\[(\d+)\]/g;
    let currentVerseNumber = '';

    while ((match = verseNumberPattern.exec(paragraph)) !== null) {
      if (match.index > lastIndex) {
        const segment = paragraph.slice(lastIndex, match.index);
        parts.push(...renderWordSegment(segment, currentVerseNumber, paragraphIndex, wordCursor, activeWordIndex, onVerseClick, onWordClick));
      }

      const verseNumber = match[1];
      currentVerseNumber = verseNumber;

      parts.push(
        <sup
          key={`verse-${paragraphIndex}-${verseNumber}`}
          className="mr-1 cursor-pointer font-sans text-[0.7rem] font-semibold uppercase tracking-[0.2em] text-gold transition hover:text-[#ffd06d]"
          onClick={() => onVerseClick?.(verseNumber)}
        >
          {verseNumber}
        </sup>,
      );
      parts.push(' ');

      lastIndex = match.index + match[0].length;
    }

    if (lastIndex < paragraph.length) {
      const tail = paragraph.slice(lastIndex);
      parts.push(...renderWordSegment(tail, currentVerseNumber, paragraphIndex, wordCursor, activeWordIndex, onVerseClick, onWordClick));
    }

    return (
      <p key={`paragraph-${paragraphIndex}`} className="mb-5 leading-8 text-[1.05rem]">
        {parts}
      </p>
    );
  });
}

export function getTotalWordCount(content: string): number {
  const cleaned = content.replace(/¶/g, '').trim();
  const paragraphs = cleaned.split(/\n+/);
  let count = 0;
  for (const p of paragraphs) {
    const noVerseNums = p.replace(/\[\d+\]/g, '');
    count += noVerseNums.trim().split(/\s+/).filter(Boolean).length;
  }
  return count;
}

export default function Reader({
  chapter,
  activeWordIndex,
  readerLabel,
  compact = false,
  sharedScroll = false,
  onVerseClick,
  onWordClick,
}: Props) {
  return (
    <div
      className={`mx-auto flex w-full flex-col ${sharedScroll ? 'min-h-full' : 'h-full overflow-y-auto'} ${
        compact ? 'px-4 py-4' : 'max-w-4xl px-7 py-8'
      }`}
    >
      <div className={`rounded-[2rem] border border-border/80 bg-surface/70 shadow-panel backdrop-blur-xl ${compact ? 'mb-4 p-5' : 'mb-6 p-6'}`}>
        <div className="mb-3 flex flex-wrap items-center gap-3">
          {readerLabel && (
            <span className="rounded-full border border-gold/40 bg-gold/10 px-3 py-1 text-[0.68rem] font-semibold uppercase tracking-[0.22em] text-gold">
              {readerLabel}
            </span>
          )}
          <span className="text-xs uppercase tracking-[0.24em] text-muted">
            {chapter.verseCount} verses
          </span>
        </div>
        <h1 className={`font-display text-text ${compact ? 'text-3xl' : 'text-4xl'}`}>{chapter.reference}</h1>
      </div>

      <article className={`rounded-[2rem] border border-border/80 bg-surface/60 font-serif shadow-panel backdrop-blur-xl ${compact ? 'px-5 py-6 text-[0.98rem]' : 'px-7 py-8'}`}>
        {parseContent(chapter.content, activeWordIndex, onVerseClick, onWordClick)}
        <p className="mt-10 border-t border-border/70 pt-5 font-sans text-xs uppercase tracking-[0.18em] text-muted">
          {chapter.copyright}
        </p>
      </article>
    </div>
  );
}
