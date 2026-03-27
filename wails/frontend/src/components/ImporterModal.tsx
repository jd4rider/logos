import { useState, useEffect } from 'react';
import { Events } from '@wailsio/runtime';
import { LogosService } from '../bindings';

interface Props {
  onClose: () => void;
  onImported: () => void;
}

export default function ImporterModal({ onClose, onImported }: Props) {
  const [tab, setTab] = useState<'url' | 'file'>('url');
  const [url, setUrl] = useState('');
  const [filePath, setFilePath] = useState('');
  const [name, setName] = useState('');
  const [abbr, setAbbr] = useState('');
  const [lang, setLang] = useState('eng');
  const [log, setLog] = useState<string[]>([]);
  const [running, setRunning] = useState(false);
  const [done, setDone] = useState(false);

  useEffect(() => {
    const unsubProgress = Events.On('import:progress', (ev) => {
      setLog((prev) => [...prev, ev.data as string]);
    });
    const unsubDone = Events.On('import:done', (ev) => {
      setLog((prev) => [...prev, ev.data as string]);
      setRunning(false);
      setDone(true);
      onImported();
    });
    const unsubError = Events.On('import:error', (ev) => {
      setLog((prev) => [...prev, '⚠ ' + (ev.data as string)]);
      setRunning(false);
    });
    return () => {
      unsubProgress();
      unsubDone();
      unsubError();
    };
  }, [onImported]);

  async function startImport() {
    setLog([]);
    setRunning(true);
    setDone(false);
    if (tab === 'url') {
      await LogosService.ImportBibleURL(url, name, abbr, lang);
    } else {
      await LogosService.ImportBibleFile(filePath, name, abbr, lang);
    }
  }

  function cancel() {
    void LogosService.CancelImport();
    setRunning(false);
  }

  async function pickFile() {
    const path = await LogosService.OpenFileDialog() as string;
    if (path) setFilePath(path);
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="w-full max-w-lg rounded-[2rem] border border-border/80 bg-bg shadow-panel">
        <div className="flex items-center justify-between border-b border-border/60 px-6 py-5">
          <div>
            <p className="text-xs uppercase tracking-[0.22em] text-muted">Import</p>
            <h2 className="mt-0.5 font-display text-2xl text-text">Bible Importer</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-full border border-border px-3 py-1 text-xs text-muted hover:text-text"
          >
            Close
          </button>
        </div>

        <div className="p-6 space-y-4">
          {/* Tabs */}
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setTab('url')}
              className={`rounded-full border px-4 py-1.5 text-sm transition ${tab === 'url' ? 'border-gold/50 bg-gold/10 text-gold' : 'border-border text-muted hover:text-text'}`}
            >
              URL Crawler
            </button>
            <button
              type="button"
              onClick={() => setTab('file')}
              className={`rounded-full border px-4 py-1.5 text-sm transition ${tab === 'file' ? 'border-gold/50 bg-gold/10 text-gold' : 'border-border text-muted hover:text-text'}`}
            >
              Local File
            </button>
          </div>

          {/* Fields */}
          <div className="space-y-3">
            {tab === 'url' ? (
              <input
                type="text"
                placeholder="https://www.biblegateway.com/passage/…"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                className="w-full rounded-[1.2rem] border border-border bg-surface/70 px-4 py-2.5 text-sm text-text placeholder:text-muted focus:border-gold/50 focus:outline-none"
              />
            ) : (
              <div className="flex gap-2">
                <input
                  type="text"
                  readOnly
                  placeholder="No file selected"
                  value={filePath}
                  className="flex-1 rounded-[1.2rem] border border-border bg-surface/70 px-4 py-2.5 text-sm text-text placeholder:text-muted"
                />
                <button
                  type="button"
                  onClick={() => void pickFile()}
                  className="rounded-[1.2rem] border border-border bg-surface/70 px-4 py-2.5 text-sm text-muted hover:text-text"
                >
                  Browse
                </button>
              </div>
            )}
            <div className="grid grid-cols-3 gap-2">
              <input
                type="text"
                placeholder="Name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="rounded-[1.2rem] border border-border bg-surface/70 px-3 py-2 text-sm text-text placeholder:text-muted focus:border-gold/50 focus:outline-none col-span-1"
              />
              <input
                type="text"
                placeholder="Abbr (e.g. NIV)"
                value={abbr}
                onChange={(e) => setAbbr(e.target.value)}
                className="rounded-[1.2rem] border border-border bg-surface/70 px-3 py-2 text-sm text-text placeholder:text-muted focus:border-gold/50 focus:outline-none"
              />
              <input
                type="text"
                placeholder="Lang (e.g. eng)"
                value={lang}
                onChange={(e) => setLang(e.target.value)}
                className="rounded-[1.2rem] border border-border bg-surface/70 px-3 py-2 text-sm text-text placeholder:text-muted focus:border-gold/50 focus:outline-none"
              />
            </div>
          </div>

          {/* Progress log */}
          {log.length > 0 && (
            <div className="max-h-36 overflow-y-auto rounded-[1.2rem] border border-border bg-surface/40 px-4 py-3 space-y-1">
              {log.map((line, i) => (
                <p key={i} className="text-xs text-muted">{line}</p>
              ))}
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-2 pt-1">
            {running ? (
              <button
                type="button"
                onClick={cancel}
                className="flex-1 rounded-full border border-red-500/40 bg-red-500/10 py-2.5 text-sm text-red-300 transition hover:bg-red-500/20"
              >
                Cancel Import
              </button>
            ) : (
              <button
                type="button"
                onClick={() => void startImport()}
                disabled={tab === 'url' ? !url.trim() : !filePath.trim()}
                className="flex-1 rounded-full border border-gold/40 bg-gold/10 py-2.5 text-sm text-gold transition hover:bg-gold/20 disabled:opacity-40"
              >
                {done ? 'Import Again' : 'Start Import'}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
