export interface LanguageOption {
  code: string;
  label: string;
}

export const languageOptions: LanguageOption[] = [
  { code: "", label: "All languages" },
  { code: "eng", label: "English" },
  { code: "spa", label: "Spanish" },
  { code: "fra", label: "French" },
  { code: "deu", label: "German" },
  { code: "por", label: "Portuguese" },
  { code: "ita", label: "Italian" },
  { code: "nld", label: "Dutch" },
  { code: "pol", label: "Polish" },
  { code: "zho", label: "Chinese" },
  { code: "hin", label: "Hindi" },
  { code: "ara", label: "Arabic" },
  { code: "kor", label: "Korean" },
  { code: "jpn", label: "Japanese" },
  { code: "vie", label: "Vietnamese" },
  { code: "ind", label: "Indonesian" },
  { code: "tur", label: "Turkish" },
  { code: "heb", label: "Hebrew" },
  { code: "grc", label: "Greek (Ancient)" },
  { code: "lat", label: "Latin" },
];

export function languageLabel(code: string) {
  if (!code) {
    return "All languages";
  }
  return languageOptions.find((option) => option.code === code)?.label ?? code;
}
