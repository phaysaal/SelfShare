import { createSignal } from 'solid-js';
import { type FileItem, listFiles as apiListFiles } from '../api/client';

export interface BreadcrumbItem {
  id: string;
  name: string;
}

const [files, setFiles] = createSignal<FileItem[]>([]);
const [currentFolder, setCurrentFolder] = createSignal('root');
const [breadcrumbs, setBreadcrumbs] = createSignal<BreadcrumbItem[]>([{ id: 'root', name: 'Home' }]);
const [loading, setLoading] = createSignal(false);

const [filteredFiles, setFilteredFilesSignal] = createSignal<FileItem[] | null>(null);

// When filteredFiles is set, it overrides the normal file listing (used for tag filtering)
export function displayFiles() {
  return filteredFiles() ?? files();
}

export function setFilteredFiles(f: FileItem[] | null) {
  setFilteredFilesSignal(f);
}

export { files, currentFolder, breadcrumbs, loading };

export async function loadFiles(folderId?: string) {
  const id = folderId || 'root';
  setCurrentFolder(id);
  setLoading(true);
  try {
    const data = await apiListFiles(id);
    setFiles(data);
  } catch (e) {
    console.error('Failed to load files:', e);
    setFiles([]);
  } finally {
    setLoading(false);
  }
}

export function navigateToFolder(id: string, name: string) {
  setBreadcrumbs((prev) => [...prev, { id, name }]);
  loadFiles(id);
}

export function navigateToBreadcrumb(index: number) {
  setBreadcrumbs((prev) => prev.slice(0, index + 1));
  const crumbs = breadcrumbs();
  loadFiles(crumbs[crumbs.length - 1].id);
}

export function navigateToRoot() {
  setBreadcrumbs([{ id: 'root', name: 'Home' }]);
  loadFiles('root');
}
