import type { ReactNode } from 'react';
import type { ChapterContent } from '../types';

interface Props {
  chapter: ChapterContent;
}

function parseContent(content: string): ReactNode[] {
  const cleaned = content.replace(/¶/g, '').replace(/\s+/g, ' ').trim();
  const paragraphs = cleaned.split(/\n+/);

  return paragraphs.map((paragraph, paragraphIndex) => {
    if (!paragraph.trim()) {
      return <br key={`break-${paragraphIndex}`} />;
    }

    const parts: ReactNode[] = [];
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    const verseNumberPattern = /\[(\d+)\]/g;

    while ((match = verseNumberPattern.exec(paragraph)) !== null) {
      if (match.index > lastIndex) {
        const segment = paragraph.slice(lastIndex, match.index).trim();
        if (segment) {
          parts.push(
            <span key={`segment-${paragraphIndex}-${lastIndex}`} className="text-text">
              {segment}{' '}
            </span>,
          );
        }
      }

      parts.push(
        <sup
          key={`verse-${paragraphIndex}-${match[1]}`}
          className="mr-1 font-sans text-[0.7rem] font-semibold uppercase tracking-[0.2em] text-gold"
        >
          {match[1]}
        </sup>,
      );

      lastIndex = match.index + match[0].length;
    }

    if (lastIndex < paragraph.length) {
      const tail = paragraph.slice(lastIndex).trim();
      if (tail) {
        parts.push(
          <span key={`tail-${paragraphIndex}`} className="text-text">
            {tail}
          </span>,
        );
      }
    }

    return (
      <p key={`paragraph-${paragraphIndex}`} className="mb-5 leading-8 text-[1.05rem]">
        {parts}
      </p>
    );
  });
}

export default function Reader({ chapter }: Props) {
  return (
    <div className="mx-auto flex h-full w-full max-w-4xl flex-col overflow-y-auto px-7 py-8">
      <div className="mb-6 rounded-[2rem] border border-border/80 bg-surface/70 p-6 shadow-panel backdrop-blur-xl">
        <div className="mb-3 flex flex-wrap items-center gap-3">
          <span className="rounded-full border border-gold/40 bg-gold/10 px-3 py-1 text-[0.68rem] font-semibold uppercase tracking-[0.22em] text-gold">
            Reader
          </span>
          <span className="text-xs uppercase tracking-[0.24em] text-muted">
            {chapter.verseCount} verses
          </span>
        </div>
        <h1 className="font-display text-4xl text-text">{chapter.reference}</h1>
      </div>

      <article className="rounded-[2rem] border border-border/80 bg-surface/60 px-7 py-8 font-serif shadow-panel backdrop-blur-xl">
        {parseContent(chapter.content)}
        <p className="mt-10 border-t border-border/70 pt-5 font-sans text-xs uppercase tracking-[0.18em] text-muted">
          {chapter.copyright}
        </p>
      </article>
    </div>
  );
}
