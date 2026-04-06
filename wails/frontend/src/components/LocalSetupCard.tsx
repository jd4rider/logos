import { useEffect, useState } from 'react';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { LogosService } from '../bindings';
import type { LocalSetupStatus } from '../types';

function explainError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function statusText(status: LocalSetupStatus) {
  const checks = [
    status.pythonReady ? 'Python ready' : 'Python missing',
    status.kokoroReady ? 'Kokoro ready' : 'Kokoro missing',
    status.ollamaRunning ? 'Ollama running' : status.ollamaInstalled ? 'Ollama stopped' : 'Ollama missing',
  ];
  return checks.join(' • ');
}

export default function LocalSetupCard() {
  const [status, setStatus] = useState<LocalSetupStatus | null>(null);
  const [launchMessage, setLaunchMessage] = useState('');
  const [error, setError] = useState('');
  const [launching, setLaunching] = useState(false);

  async function refreshStatus() {
    try {
      const next = (await LogosService.GetLocalSetupStatus()) as LocalSetupStatus;
      setStatus(next);
    } catch (loadError) {
      setError(explainError(loadError));
    }
  }

  useEffect(() => {
    void refreshStatus();
  }, []);

  async function handlePrepare() {
    setError('');
    setLaunchMessage('');
    setLaunching(true);
    try {
      await LogosService.RunSetupScript();
      setLaunchMessage('Setup opened in a terminal window. When it finishes, restart Logos AI if the new voices or models do not appear automatically.');
    } catch (launchError) {
      setError(explainError(launchError));
    } finally {
      setLaunching(false);
      void refreshStatus();
    }
  }

  if (!status?.needsSetup && !launchMessage && !error) {
    return null;
  }

  return (
    <div className="rounded-[1.2rem] border border-gold/25 bg-gold/8 px-4 py-4 text-sm text-text">
      <div className="flex flex-wrap items-center gap-2">
        <span className="rounded-full border border-gold/35 bg-gold/10 px-3 py-1 text-[0.65rem] uppercase tracking-[0.2em] text-gold">
          Local Setup
        </span>
        {status && <span className="text-[0.72rem] uppercase tracking-[0.18em] text-muted">{statusText(status)}</span>}
      </div>

      <p className="mt-3 leading-7 text-muted">
        Prepare this machine for local voices and AI. Logos AI will set up a private Python runtime, download Kokoro,
        and pull the recommended Ollama models
        {status ? ` (${status.chatModel} + ${status.embedModel}).` : '.'}
      </p>

      {launchMessage && <p className="mt-3 text-xs leading-6 text-gold">{launchMessage}</p>}
      {error && <p className="mt-3 text-xs leading-6 text-red-300">{error}</p>}

      <div className="mt-4 flex flex-wrap gap-3">
        <button
          type="button"
          onClick={() => void handlePrepare()}
          disabled={launching}
          className="rounded-full border border-gold/40 bg-gold/10 px-4 py-2 text-sm text-gold transition hover:bg-gold/20 disabled:opacity-50"
        >
          {launching ? 'Opening setup...' : 'Prepare this device'}
        </button>
        <button
          type="button"
          onClick={() => BrowserOpenURL('https://ollama.com/download')}
          className="rounded-full border border-border bg-bg/45 px-4 py-2 text-sm text-text transition hover:border-gold/35 hover:text-gold"
        >
          Ollama download
        </button>
        <button
          type="button"
          onClick={() => BrowserOpenURL('https://docs.logos-ai.online/setup/voices/')}
          className="rounded-full border border-border bg-bg/45 px-4 py-2 text-sm text-text transition hover:border-gold/35 hover:text-gold"
        >
          Voice docs
        </button>
      </div>
    </div>
  );
}
