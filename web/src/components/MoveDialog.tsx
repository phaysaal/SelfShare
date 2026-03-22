import { createSignal, onMount, For, Show } from 'solid-js';
import { type FileItem, listFiles, apiFetch } from '../api/client';

interface Props {
  file: FileItem | null;
  onClose: () => void;
  onMoved: () => void;
}

interface FolderEntry {
  id: string;
  name: string;
}

export default function MoveDialog(props: Props) {
  const [folders, setFolders] = createSignal<FileItem[]>([]);
  const [currentId, setCurrentId] = createSignal('root');
  const [breadcrumbs, setBreadcrumbs] = createSignal<FolderEntry[]>([{ id: 'root', name: 'Home' }]);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal('');
  const [moving, setMoving] = createSignal(false);

  onMount(() => loadFolder('root'));

  async function loadFolder(id: string) {
    setLoading(true);
    setCurrentId(id);
    try {
      const all = await listFiles(id);
      // Only show folders, exclude the file being moved
      setFolders(all.filter((f) => f.is_dir && f.id !== props.file?.id));
    } catch {
      setFolders([]);
    }
    setLoading(false);
  }

  function openFolder(folder: FileItem) {
    setBreadcrumbs((prev) => [...prev, { id: folder.id, name: folder.name }]);
    loadFolder(folder.id);
  }

  function navigateTo(index: number) {
    setBreadcrumbs((prev) => prev.slice(0, index + 1));
    const crumbs = breadcrumbs();
    loadFolder(crumbs[crumbs.length - 1].id);
  }

  async function doMove() {
    if (!props.file) return;
    // Don't move to same parent
    if (currentId() === props.file.parent_id) {
      setError('File is already in this folder');
      return;
    }
    setMoving(true);
    setError('');
    try {
      const resp = await apiFetch(`/files/${props.file.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ parent_id: currentId() }),
      });
      if (!resp.ok) {
        const data = await resp.json();
        throw new Error(data.error || 'Move failed');
      }
      props.onMoved();
      props.onClose();
    } catch (e: any) {
      setError(e.message);
    }
    setMoving(false);
  }

  function handleOverlayClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('dialog-overlay')) props.onClose();
  }

  return (
    <Show when={props.file}>
      <div class="dialog-overlay" onClick={handleOverlayClick}>
        <div class="dialog-card" style={{ "max-width": "500px" }}>
          <h2>Move "{props.file!.name}"</h2>
          <p style={{ color: '#888', "font-size": '13px', "margin-bottom": '16px' }}>Select destination folder:</p>

          {/* Breadcrumbs */}
          <div class="move-breadcrumbs">
            <For each={breadcrumbs()}>
              {(crumb, index) => (
                <>
                  {index() > 0 && <span class="sep">/</span>}
                  {index() === breadcrumbs().length - 1 ? (
                    <strong>{crumb.name}</strong>
                  ) : (
                    <a onClick={() => navigateTo(index())}>{crumb.name}</a>
                  )}
                </>
              )}
            </For>
          </div>

          {/* Folder list */}
          <div class="move-folder-list">
            <Show when={loading()}>
              <div class="move-empty">Loading...</div>
            </Show>
            <Show when={!loading() && folders().length === 0}>
              <div class="move-empty">No subfolders here</div>
            </Show>
            <Show when={!loading()}>
              <For each={folders()}>
                {(folder) => (
                  <div class="move-folder-item" onDblClick={() => openFolder(folder)}>
                    <span class="move-folder-icon">{'\u{1F4C1}'}</span>
                    <span class="move-folder-name">{folder.name}</span>
                    <button class="btn-icon" onClick={() => openFolder(folder)}>Open</button>
                  </div>
                )}
              </For>
            </Show>
          </div>

          {error() && <div class="error-text">{error()}</div>}

          <div class="dialog-actions">
            <button class="btn-secondary" onClick={props.onClose}>Cancel</button>
            <button class="btn-primary" onClick={doMove} disabled={moving()}>
              {moving() ? 'Moving...' : `Move here`}
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}
