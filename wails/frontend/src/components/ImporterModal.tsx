import { useState, useEffect } from 'react';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { ImportBibleURL, ImportBibleFile, CancelImport, OpenFileDialog } from '../wailsjs/go/main/App';

interface Props {
  onClose: () => void;
}

type Mode = 'url' | 'file';

export default function ImporterModal({ onClose }: Props) {
  const [mode, setMode] = useState<Mode>('url');
  const [url, setUrl] = useState('');
  const [filePath, setFilePath] = useState('');
  const [name, setName] = useState('');
  const [abbr, setAbbr] = useState('');
  const [lang, setLang] = useState('eng');
  const [importing, setImporting] = useState(false);
  const [done, setDone] = useState(false);
  const [progress, setProgress] = useState<string[]>([]);
  const [errorMsg, setErrorMsg] = useState('');

  useEffect(() => {
    const onProgress = (msg: string) => setProgress(p => [...p, msg]);
    const onDone = () => { setImporting(false); setDone(true); };
    const onError = (err: string) => { setImporting(false); setErrorMsg(err); };

    EventsOn('import:progress', onProgress);
    EventsOn('import:done', onDone);
    EventsOn('import:error', onError);
    return () => {
      EventsOff('import:progress');
      EventsOff('import:done');
      EventsOff('import:error');
    };
  }, []);

  const handleBrowse = async () => {
    try {
      const path = await OpenFileDialog();
      if (path) setFilePath(path);
    } catch (e) {
      console.error(e);
    }
  };

  const handleImport = () => {
    setImporting(true);
    setDone(false);
    setErrorMsg('');
    setProgress([]);
    if (mode === 'url') {
      ImportBibleURL(url, name, abbr, lang);
    } else {
      ImportBibleFile(filePath, name, abbr, lang);
    }
  };

  const handleCancel = () => {
    CancelImport();
    setImporting(false);
  };

  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50">
      <div className="bg-surface border border-border rounded-lg w-[520px] max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3 border-b border-border">
          <span className="text-gold font-semibold">📥 Import Bible</span>
          <button onClick={onClose} className="text-muted hover:text-text text-xl">×</button>
        </div>

        {/* Mode toggle */}
        <div className="flex px-5 pt-4 gap-2">
          <button
            onClick={() => setMode('url')}
            className={`px-3 py-1 text-sm rounded border transition-colors ${mode === 'url' ? 'border-gold text-gold bg-highlight' : 'border-border text-muted hover:border-gold'}`}
          >
            From URL
          </button>
          <button
            onClick={() => setMode('file')}
            className={`px-3 py-1 text-sm rounded border transition-colors ${mode === 'file' ? 'border-gold text-gold bg-highlight' : 'border-border text-muted hover:border-gold'}`}
          >
            From File
          </button>
        </div>

        {/* Form */}
        <div className="px-5 py-4 space-y-3 overflow-y-auto flex-1">
          {mode === 'url' ? (
            <div>
              <label className="text-xs text-muted block mb-1">URL (e.g. BibleGateway chapter URL)</label>
              <input
                type="text"
                value={url}
                onChange={e => setUrl(e.target.value)}
                placeholder="https://www.biblegateway.com/passage/..."
                className="w-full bg-bg border border-border rounded px-3 py-1.5 text-sm text-text focus:border-gold outline-none"
              />
            </div>
          ) : (
            <div>
              <label className="text-xs text-muted block mb-1">File Path (.csv, .db, .sqlite)</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={filePath}
                  onChange={e => setFilePath(e.target.value)}
                  placeholder="/path/to/bible.csv"
                  className="flex-1 bg-bg border border-border rounded px-3 py-1.5 text-sm text-text focus:border-gold outline-none"
                />
                <button
                  onClick={handleBrowse}
                  className="px-3 py-1.5 text-sm border border-border rounded hover:border-gold text-muted hover:text-text transition-colors"
                >
                  Browse
                </button>
              </div>
            </div>
          )}

          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="text-xs text-muted block mb-1">Name</label>
              <input
                type="text"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder="King James Version"
                className="w-full bg-bg border border-border rounded px-2 py-1.5 text-sm text-text focus:border-gold outline-none"
              />
            </div>
            <div>
              <label className="text-xs text-muted block mb-1">Abbreviation</label>
              <input
                type="text"
                value={abbr}
                onChange={e => setAbbr(e.target.value)}
                placeholder="KJV"
                className="w-full bg-bg border border-border rounded px-2 py-1.5 text-sm text-text focus:border-gold outline-none"
              />
            </div>
            <div>
              <label className="text-xs text-muted block mb-1">Language</label>
              <input
                type="text"
                value={lang}
                onChange={e => setLang(e.target.value)}
                placeholder="eng"
                className="w-full bg-bg border border-border rounded px-2 py-1.5 text-sm text-text focus:border-gold outline-none"
              />
            </div>
          </div>

          {/* Progress log */}
          {(progress.length > 0 || importing) && (
            <div className="bg-bg border border-border rounded p-2 max-h-36 overflow-y-auto">
              {importing && progress.length === 0 && (
                <span className="text-gold text-xs animate-pulse">Starting import…</span>
              )}
              {progress.map((msg, i) => (
                <div key={i} className="text-xs text-muted font-mono">{msg}</div>
              ))}
            </div>
          )}

          {errorMsg && (
            <div className="bg-red-900/30 border border-red-700 rounded p-2 text-red-300 text-xs">
              ✗ {errorMsg}
            </div>
          )}

          {done && (
            <div className="bg-green-900/30 border border-green-700 rounded p-2 text-green-300 text-sm text-center">
              ✓ Import complete!
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex justify-between items-center px-5 py-3 border-t border-border">
          {importing ? (
            <button onClick={handleCancel} className="px-3 py-1.5 text-sm bg-red-900/40 text-red-300 border border-red-700 rounded hover:bg-red-900">
              Cancel Import
            </button>
          ) : done ? (
            <button onClick={onClose} className="px-4 py-1.5 text-sm bg-highlight text-gold border border-gold rounded hover:bg-gold hover:text-bg transition-colors">
              Done
            </button>
          ) : (
            <button
              onClick={handleImport}
              disabled={mode === 'url' ? !url.trim() : !filePath.trim()}
              className="px-4 py-1.5 text-sm bg-highlight text-gold border border-gold rounded hover:bg-gold hover:text-bg transition-colors disabled:opacity-50"
            >
              Import
            </button>
          )}
          <button onClick={onClose} className="px-3 py-1.5 text-sm text-muted border border-border rounded hover:text-text">
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
