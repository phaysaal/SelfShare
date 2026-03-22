import { For, Show, createSignal } from 'solid-js';
import { files, loading, currentFolder, navigateToFolder, loadFiles } from '../stores/files';
import { type FileItem, deleteFile, createFolder, uploadFile, downloadUrl } from '../api/client';

export default function FileList(props: { onViewFile: (file: FileItem, allFiles: FileItem[]) => void; onShareFile: (file: FileItem) => void }) {
  const [uploading, setUploading] = createSignal(false);
  const [uploadProgress, setUploadProgress] = createSignal<Record<string, number>>({});
  const [dragOver, setDragOver] = createSignal(false);
  let fileInputRef: HTMLInputElement | undefined;

  function isViewable(mime?: string): boolean {
    if (!mime) return false;
    return mime.startsWith('image/') || mime.startsWith('video/') || mime.startsWith('audio/') || mime === 'application/pdf';
  }

  function fileIcon(f: FileItem): string {
    if (f.is_dir) return '\u{1F4C1}';
    const m = f.mime_type || '';
    if (m.startsWith('image/')) return '\u{1F4F7}';
    if (m.startsWith('video/')) return '\u{1F3AC}';
    if (m.startsWith('audio/')) return '\u{1F3B5}';
    if (m === 'application/pdf') return '\u{1F4DC}';
    if (m === 'application/zip') return '\u{1F4E6}';
    return '\u{1F4C4}';
  }

  function formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function handleClick(f: FileItem) {
    if (f.is_dir) {
      navigateToFolder(f.id, f.name);
    } else if (isViewable(f.mime_type)) {
      props.onViewFile(f, files());
    }
  }

  async function handleDelete(f: FileItem) {
    if (!confirm(`Delete "${f.name}"?`)) return;
    try {
      await deleteFile(f.id);
      loadFiles(currentFolder());
    } catch (e: any) {
      alert(e.message);
    }
  }

  async function handleNewFolder() {
    const name = prompt('Folder name:');
    if (!name) return;
    try {
      await createFolder(currentFolder(), name);
      loadFiles(currentFolder());
    } catch (e: any) {
      alert(e.message);
    }
  }

  async function handleUpload(fileList: FileList) {
    if (!fileList.length) return;
    setUploading(true);
    const progress: Record<string, number> = {};

    for (const file of Array.from(fileList)) {
      progress[file.name] = 0;
      setUploadProgress({ ...progress });
      try {
        await uploadFile(currentFolder(), file, (pct) => {
          progress[file.name] = pct;
          setUploadProgress({ ...progress });
        });
        progress[file.name] = 100;
        setUploadProgress({ ...progress });
      } catch (e: any) {
        console.error('Upload failed:', file.name, e);
      }
    }

    setUploading(false);
    setUploadProgress({});
    loadFiles(currentFolder());
  }

  function onDrop(e: DragEvent) {
    e.preventDefault();
    setDragOver(false);
    if (e.dataTransfer?.files) handleUpload(e.dataTransfer.files);
  }

  return (
    <div
      class={dragOver() ? 'file-list-container drag-over' : 'file-list-container'}
      onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
      onDragLeave={() => setDragOver(false)}
      onDrop={onDrop}
    >
      <div class="toolbar">
        <label class="btn-secondary">
          <input
            ref={fileInputRef}
            type="file"
            multiple
            hidden
            onChange={(e) => e.currentTarget.files && handleUpload(e.currentTarget.files)}
          />
          Upload Files
        </label>
        <button class="btn-secondary" onClick={handleNewFolder}>New Folder</button>
      </div>

      <Show when={uploading()}>
        <div class="upload-progress">
          <For each={Object.entries(uploadProgress())}>
            {([name, pct]) => (
              <div class="upload-item">
                <span class="upload-name">{name}</span>
                <div class="progress-bar">
                  <div class="progress-fill" style={{ width: `${pct}%` }} />
                </div>
                <span class="upload-pct">{pct}%</span>
              </div>
            )}
          </For>
        </div>
      </Show>

      <Show when={loading()}>
        <div class="empty">Loading...</div>
      </Show>

      <Show when={!loading() && files().length === 0}>
        <div class="empty">
          This folder is empty. Upload files or create a folder to get started.
          <br /><br />
          <small>You can also drag and drop files here.</small>
        </div>
      </Show>

      <Show when={!loading() && files().length > 0}>
        <ul class="file-list">
          <For each={files()}>
            {(f) => (
              <li class="file-item" onClick={() => handleClick(f)}>
                <div class="file-icon">{fileIcon(f)}</div>
                <div class="file-info">
                  <div class="file-name">{f.name}</div>
                  <div class="file-meta">
                    {!f.is_dir && formatSize(f.size_bytes)}
                    {!f.is_dir && ' \u00B7 '}
                    {new Date(f.created_at).toLocaleDateString()}
                  </div>
                </div>
                <div class="file-actions">
                  <Show when={isViewable(f.mime_type) && !f.is_dir}>
                    <button class="btn-icon" onClick={(e) => { e.stopPropagation(); props.onViewFile(f, files()); }}>View</button>
                  </Show>
                  <Show when={!f.is_dir}>
                    <button class="btn-icon" onClick={(e) => { e.stopPropagation(); props.onShareFile(f); }}>Share</button>
                    <button class="btn-icon" onClick={(e) => { e.stopPropagation(); window.open(downloadUrl(f.id)); }}>Download</button>
                  </Show>
                  <button class="btn-icon danger" onClick={(e) => { e.stopPropagation(); handleDelete(f); }}>Delete</button>
                </div>
              </li>
            )}
          </For>
        </ul>
      </Show>
    </div>
  );
}
