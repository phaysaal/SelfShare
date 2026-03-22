import { Show, createSignal } from 'solid-js';
import { isLoggedIn } from './stores/auth';
import { loadFiles } from './stores/files';
import { type FileItem } from './api/client';
import Login from './pages/Login';
import Header from './components/Header';
import Breadcrumbs from './components/Breadcrumbs';
import FileList from './components/FileList';
import PhotoGallery from './components/PhotoGallery';
import MediaViewer from './components/MediaViewer';
import ShareDialog from './components/ShareDialog';

export default function App() {
  const [activeTab, setActiveTab] = createSignal('files');
  const [viewerFile, setViewerFile] = createSignal<FileItem | null>(null);
  const [viewerFiles, setViewerFiles] = createSignal<FileItem[]>([]);
  const [shareFile, setShareFile] = createSignal<FileItem | null>(null);

  function handleTabChange(tab: string) {
    setActiveTab(tab);
    if (tab === 'files') loadFiles();
  }

  function openViewer(file: FileItem, allFiles: FileItem[]) {
    setViewerFiles(allFiles);
    setViewerFile(file);
  }

  return (
    <>
      <Show when={!isLoggedIn()}>
        <Login />
      </Show>

      <Show when={isLoggedIn()}>
        <Header activeTab={activeTab()} onTabChange={handleTabChange} />

        <main class="main-content">
          <Show when={activeTab() === 'files'}>
            <Breadcrumbs />
            <FileList onViewFile={openViewer} onShareFile={(f) => setShareFile(f)} />
          </Show>

          <Show when={activeTab() === 'photos'}>
            <PhotoGallery onViewPhoto={openViewer} />
          </Show>
        </main>

        <MediaViewer
          file={viewerFile()}
          allFiles={viewerFiles()}
          onClose={() => { setViewerFile(null); setViewerFiles([]); }}
        />

        <ShareDialog
          file={shareFile()}
          onClose={() => setShareFile(null)}
        />
      </Show>
    </>
  );
}
