export interface VerseSpeechSegment {
  number: string;
  rawStart: number;
  startWordIndex: number;
  wordCount: number;
  content: string;
}

const versePattern = /\[(\d+)\]/g;

function normalizeVerseText(text: string): string {
  return text.replace(/¶/g, ' ').replace(/\s+/g, ' ').trim();
}

function countWords(text: string): number {
  const normalized = normalizeVerseText(text);
  if (!normalized) {
    return 0;
  }
  return normalized.split(/\s+/).filter(Boolean).length;
}

export function parseVerseSpeechSegments(content: string): VerseSpeechSegment[] {
  const matches = Array.from(content.matchAll(versePattern));
  if (matches.length === 0) {
    const normalized = normalizeVerseText(content);
    if (!normalized) {
      return [];
    }
    return [
      {
        number: '1',
        rawStart: 0,
        startWordIndex: 0,
        wordCount: countWords(normalized),
        content: normalized,
      },
    ];
  }

  let totalWords = countWords(content.slice(0, matches[0].index ?? 0));
  const segments: VerseSpeechSegment[] = [];

  matches.forEach((match, index) => {
    const rawStart = match.index ?? 0;
    const nextStart = matches[index + 1]?.index ?? content.length;
    const verseText = normalizeVerseText(content.slice(rawStart + match[0].length, nextStart));
    const wordCount = countWords(verseText);

    segments.push({
      number: match[1],
      rawStart,
      startWordIndex: totalWords,
      wordCount,
      content: verseText,
    });

    totalWords += wordCount;
  });

  return segments;
}

export function buildSpeechSliceFromVerse(content: string, verseNumber?: string | null): string {
  if (!verseNumber) {
    return content;
  }

  const segment = parseVerseSpeechSegments(content).find((item) => item.number === verseNumber);
  if (!segment) {
    return content;
  }

  return content.slice(segment.rawStart);
}

export function findVerseForWordIndex(segments: VerseSpeechSegment[], wordIndex: number): VerseSpeechSegment | null {
  if (segments.length === 0) {
    return null;
  }

  if (wordIndex < segments[0].startWordIndex) {
    return null;
  }

  for (const segment of segments) {
    const limit = segment.startWordIndex + Math.max(segment.wordCount, 1);
    if (wordIndex < limit) {
      return segment;
    }
  }

  return segments[segments.length - 1];
}
