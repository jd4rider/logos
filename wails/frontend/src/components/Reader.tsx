import { useMemo } from 'react';
import type { ChapterContent } from '../types';

interface Props {
  chapter: ChapterContent;
  highlightWordIndex?: number;
}

const verseNumRe = /\[(\d+)\]/g;

function parseContent(content: string, highlightWordIndex: number) {
  // Remove pilcrow
  const cleaned = content.replace(/¶/g, '').replace(/\s+/g, ' ').trim();
  const paragraphs = cleaned.split(/\n+/);
  let wordCount = 0;

  return paragraphs.map((para, pIdx) => {
    if (!para.trim()) return <br key={pIdx} />;

    const parts: React.ReactNode[] = [];
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    const re = /\[(\d+)\]/g;

    while ((match = re.exec(para)) !== null) {
      // Text before verse number
      if (match.index > lastIndex) {
        const segment = para.slice(lastIndex, match.index);
        const words = segment.split(/\s+/).filter(Boolean);
        words.forEach((w, i) => {
          const wi = wordCount++;
          parts.push(
            <span key={`w-${wi}`}
              className={wi === highlightWordIndex ? 'bg-gold text-bg font-bold' : 'text-text'}
            >
              {w}{' '}
            </span>
          );
        });
      }
      // Verse number
      parts.push(
        <sup key={`v-${match[1]}`} className="text-gold font-bold text-xs mr-1 select-none">
          {match[1]}
        </sup>
      );
      lastIndex = match.index + match[0].length;
    }

    // Remaining text
    if (lastIndex < para.length) {
      const segment = para.slice(lastIndex);
      const words = segment.split(/\s+/).filter(Boolean);
      words.forEach((w) => {
        const wi = wordCount++;
        parts.push(
          <span key={`w-${wi}`}
            className={wi === highlightWordIndex ? 'bg-gold text-bg font-bold' : 'text-text'}
          >
            {w}{' '}
          </span>
        );
      });
    }

    return <p key={pIdx} className="mb-4 leading-8">{parts}</p>;
  });
}

export default function Reader({ chapter, highlightWordIndex = -1 }: Props) {
  const content = useMemo(
    () => parseContent(chapter.content, highlightWordIndex),
    [chapter.content, highlightWordIndex]
  );

  return (
    <div className="flex-1 overflow-y-auto px-8 py-6 max-w-3xl mx-auto w-full">
      <h1 className="text-accent font-bold text-xl mb-6">{chapter.reference}</h1>
      <div className="font-serif text-lg">{content}</div>
      <p className="mt-8 text-muted text-xs italic border-t border-border pt-4">
        {chapter.copyright}
      </p>
    </div>
  );
}
