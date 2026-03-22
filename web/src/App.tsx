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

export default function App() {
  const [activeTab, setActiveTab] = createSignal('files');
  const [viewerFile, setViewerFile] = createSignal<FileItem | null>(null);
  const [viewerFiles, setViewerFiles] = createSignal<FileItem[]>([]);

  function handleTabChange(tab: string) {
    setActiveTab(tab);
    if (tab === 'files') loadFiles();
  }

  function openViewer(file: FileItem, allFiles: FileItem[]) {
    setViewerFiles(allFiles);
    setViewerFile(file);
  }

  function closeViewer() {
    setViewerFile(null);
    setViewerFiles([]);
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
            <FileList onViewFile={openViewer} />
          </Show>

          <Show when={activeTab() === 'photos'}>
            <PhotoGallery onViewPhoto={openViewer} />
          </Show>
        </main>

        <MediaViewer file={viewerFile()} allFiles={viewerFiles()} onClose={closeViewer} />
      </Show>
    </>
  );
}
