import { createSignal, Show } from 'solid-js';
import { type FileItem, type ShareInfo, createShare } from '../api/client';

interface Props {
  file: FileItem | null;
  onClose: () => void;
}

export default function ShareDialog(props: Props) {
  const [password, setPassword] = createSignal('');
  const [expiresIn, setExpiresIn] = createSignal('');
  const [maxDownloads, setMaxDownloads] = createSignal('');
  const [result, setResult] = createSignal<ShareInfo | null>(null);
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [copied, setCopied] = createSignal(false);

  async function handleCreate() {
    if (!props.file) return;
    setLoading(true);
    setError('');
    try {
      const expSec = expiresIn() ? parseInt(expiresIn()) * 3600 : undefined;
      const maxDl = maxDownloads() ? parseInt(maxDownloads()) : undefined;
      const share = await createShare(props.file.id, password() || undefined, expSec, maxDl);
      // Build full URL
      share.url = window.location.origin + '/s/' + share.token;
      setResult(share);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  function copyLink() {
    const r = result();
    if (!r) return;
    navigator.clipboard.writeText(r.url);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  function handleOverlayClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('dialog-overlay')) {
      props.onClose();
    }
  }

  return (
    <Show when={props.file}>
      <div class="dialog-overlay" onClick={handleOverlayClick}>
        <div class="dialog-card">
          <Show when={!result()} fallback={
            <div class="share-result">
              <h2>Link Created</h2>
              <p class="share-filename">{props.file!.name}</p>
              <div class="share-url-box">
                <input type="text" value={result()!.url} readonly onClick={(e) => (e.target as HTMLInputElement).select()} />
                <button class="btn-primary" onClick={copyLink}>
                  {copied() ? 'Copied!' : 'Copy'}
                </button>
              </div>
              {result()!.has_password && <p class="share-note">Password protected</p>}
              {result()!.expires_at && <p class="share-note">Expires: {new Date(result()!.expires_at!).toLocaleString()}</p>}
              {result()!.max_downloads && <p class="share-note">Max downloads: {result()!.max_downloads}</p>}
              <button class="btn-secondary full-width" onClick={props.onClose} style="margin-top: 16px">Done</button>
            </div>
          }>
            <h2>Share "{props.file!.name}"</h2>
            <div class="share-form">
              <div class="field">
                <label>Password (optional)</label>
                <input
                  type="password"
                  value={password()}
                  onInput={(e) => setPassword(e.currentTarget.value)}
                  placeholder="Leave empty for no password"
                />
              </div>
              <div class="field">
                <label>Expires in (hours, optional)</label>
                <input
                  type="number"
                  value={expiresIn()}
                  onInput={(e) => setExpiresIn(e.currentTarget.value)}
                  placeholder="e.g. 24, 168"
                  min="1"
                />
              </div>
              <div class="field">
                <label>Max downloads (optional)</label>
                <input
                  type="number"
                  value={maxDownloads()}
                  onInput={(e) => setMaxDownloads(e.currentTarget.value)}
                  placeholder="e.g. 10"
                  min="1"
                />
              </div>
              {error() && <div class="error-text">{error()}</div>}
              <div class="dialog-actions">
                <button class="btn-secondary" onClick={props.onClose}>Cancel</button>
                <button class="btn-primary" onClick={handleCreate} disabled={loading()}>
                  {loading() ? 'Creating...' : 'Create Link'}
                </button>
              </div>
            </div>
          </Show>
        </div>
      </div>
    </Show>
  );
}
