import { Show, createSignal } from 'solid-js';
import { isLoggedIn } from './stores/auth';
import { loadFiles, setFilteredFiles } from './stores/files';
import { type FileItem, type Tag, listFilesByTag } from './api/client';
import Login from './pages/Login';
import Header from './components/Header';
import Breadcrumbs from './components/Breadcrumbs';
import FileList from './components/FileList';
import PhotoGallery from './components/PhotoGallery';
import MediaViewer from './components/MediaViewer';
import ShareDialog from './components/ShareDialog';
import { TagSidebar } from './components/TagManager';

export default function App() {
  const [activeTab, setActiveTab] = createSignal('files');
  const [viewerFile, setViewerFile] = createSignal<FileItem | null>(null);
  const [viewerFiles, setViewerFiles] = createSignal<FileItem[]>([]);
  const [shareFile, setShareFile] = createSignal<FileItem | null>(null);
  const [selectedTag, setSelectedTag] = createSignal<string | null>(null);

  function handleTabChange(tab: string) {
    setActiveTab(tab);
    if (tab === 'files') {
      setSelectedTag(null);
      setFilteredFiles(null);
      loadFiles();
    }
  }

  function openViewer(file: FileItem, allFiles: FileItem[]) {
    setViewerFiles(allFiles);
    setViewerFile(file);
  }

  async function handleSelectTag(tag: Tag | null) {
    if (tag) {
      setSelectedTag(tag.id);
      const files = await listFilesByTag(tag.id);
      setFilteredFiles(files);
    } else {
      setSelectedTag(null);
      setFilteredFiles(null);
      loadFiles();
    }
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
            <div class="files-layout">
              <div class="files-sidebar">
                <TagSidebar onSelectTag={handleSelectTag} selectedTagId={selectedTag()} />
              </div>
              <div class="files-main">
                <Show when={!selectedTag()}>
                  <Breadcrumbs />
                </Show>
                <FileList onViewFile={openViewer} onShareFile={(f) => setShareFile(f)} />
              </div>
            </div>
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
