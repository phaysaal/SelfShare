import { Show, createSignal, createEffect, onCleanup } from 'solid-js';
import { type FileItem, viewUrl, downloadUrl } from '../api/client';

interface Props {
  file: FileItem | null;
  allFiles: FileItem[];
  onClose: () => void;
}

export default function MediaViewer(props: Props) {
  const [currentIndex, setCurrentIndex] = createSignal(-1);

  function viewableFiles() {
    return props.allFiles.filter(
      (f) => !f.is_dir && isViewable(f.mime_type)
    );
  }

  function currentFile() {
    const files = viewableFiles();
    const idx = currentIndex();
    return idx >= 0 && idx < files.length ? files[idx] : null;
  }

  createEffect(() => {
    if (props.file) {
      const files = viewableFiles();
      const idx = files.findIndex((f) => f.id === props.file!.id);
      setCurrentIndex(idx >= 0 ? idx : 0);
    }
  });

  function navigate(dir: number) {
    const files = viewableFiles();
    if (files.length < 2) return;
    setCurrentIndex((prev) => {
      let next = prev + dir;
      if (next < 0) next = files.length - 1;
      if (next >= files.length) next = 0;
      return next;
    });
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (!props.file) return;
    if (e.key === 'Escape') props.onClose();
    if (e.key === 'ArrowLeft') navigate(-1);
    if (e.key === 'ArrowRight') navigate(1);
  }

  createEffect(() => {
    if (props.file) {
      document.addEventListener('keydown', handleKeyDown);
      document.body.style.overflow = 'hidden';
    } else {
      document.removeEventListener('keydown', handleKeyDown);
      document.body.style.overflow = '';
    }
  });

  onCleanup(() => {
    document.removeEventListener('keydown', handleKeyDown);
    document.body.style.overflow = '';
  });

  function handleOverlayClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains('viewer-overlay')) {
      props.onClose();
    }
  }

  return (
    <Show when={props.file && currentFile()}>
      <div class="viewer-overlay" onClick={handleOverlayClick}>
        <div class="viewer-topbar">
          <div class="viewer-title">
            {currentFile()!.name}
            <span class="viewer-size">{formatSize(currentFile()!.size_bytes)}</span>
          </div>
          <div class="viewer-controls">
            <button onClick={() => window.open(downloadUrl(currentFile()!.id))}>Download</button>
            <button onClick={props.onClose}>Close</button>
          </div>
        </div>

        <Show when={viewableFiles().length > 1}>
          <button class="viewer-nav prev" onClick={() => navigate(-1)}>{'\u2039'}</button>
          <button class="viewer-nav next" onClick={() => navigate(1)}>{'\u203A'}</button>
        </Show>

        <div class="viewer-content">
          {renderMedia(currentFile()!)}
        </div>
      </div>
    </Show>
  );
}

function isViewable(mime?: string): boolean {
  if (!mime) return false;
  return mime.startsWith('image/') || mime.startsWith('video/') || mime.startsWith('audio/') || mime === 'application/pdf';
}

function renderMedia(f: FileItem) {
  const url = viewUrl(f.id);
  const mime = f.mime_type || '';

  if (mime.startsWith('image/')) {
    return <img src={url} alt={f.name} />;
  }
  if (mime.startsWith('video/')) {
    return (
      <video controls autoplay playsinline>
        <source src={url} type={mime} />
      </video>
    );
  }
  if (mime.startsWith('audio/')) {
    return (
      <audio controls autoplay>
        <source src={url} type={mime} />
      </audio>
    );
  }
  if (mime === 'application/pdf') {
    return <iframe src={url} />;
  }
  return null;
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}
