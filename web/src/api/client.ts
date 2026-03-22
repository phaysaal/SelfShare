const API = '/api/v1';

let accessToken: string | null = localStorage.getItem('access_token');
let refreshToken: string | null = localStorage.getItem('refresh_token');
let onAuthFailure: (() => void) | null = null;

export function setAuthFailureHandler(handler: () => void) {
  onAuthFailure = handler;
}

export function setTokens(access: string, refresh: string) {
  accessToken = access;
  refreshToken = refresh;
  localStorage.setItem('access_token', access);
  localStorage.setItem('refresh_token', refresh);
}

export function clearTokens() {
  accessToken = null;
  refreshToken = null;
  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
  localStorage.removeItem('user');
}

export function getAccessToken() {
  return accessToken;
}

async function tryRefresh(): Promise<boolean> {
  if (!refreshToken) return false;
  try {
    const resp = await fetch(`${API}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!resp.ok) return false;
    const data = await resp.json();
    setTokens(data.access_token, data.refresh_token);
    return true;
  } catch {
    return false;
  }
}

export async function apiFetch(path: string, opts: RequestInit = {}): Promise<Response> {
  const headers: Record<string, string> = { ...(opts.headers as Record<string, string> || {}) };
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`;
  opts.headers = headers;

  let resp = await fetch(`${API}${path}`, opts);

  if (resp.status === 401 && refreshToken) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      headers['Authorization'] = `Bearer ${accessToken}`;
      opts.headers = headers;
      resp = await fetch(`${API}${path}`, opts);
    } else {
      onAuthFailure?.();
      throw new Error('Session expired');
    }
  }

  return resp;
}

export function viewUrl(fileId: string): string {
  return `${API}/files/${fileId}/view?token=${encodeURIComponent(accessToken || '')}`;
}

export function downloadUrl(fileId: string): string {
  return `${API}/files/${fileId}/download?token=${encodeURIComponent(accessToken || '')}`;
}

export function thumbUrl(fileId: string, size: 'sm' | 'md' | 'lg' = 'sm'): string {
  return `${API}/files/${fileId}/thumb?size=${size}&token=${encodeURIComponent(accessToken || '')}`;
}

// --- Typed API methods ---

export interface FileItem {
  id: string;
  parent_id: string | null;
  name: string;
  is_dir: boolean;
  size_bytes: number;
  mime_type?: string;
  sha256?: string;
  created_at: string;
  updated_at: string;
}

export interface PhotoItem extends FileItem {
  taken_at?: string;
  camera_make?: string;
  camera_model?: string;
  width?: number;
  height?: number;
  thumb_url?: string;
}

export interface TimelineGroup {
  year: number;
  month: number;
  label: string;
  count: number;
}

export interface UserInfo {
  id: string;
  username: string;
  display_name: string;
  is_admin: boolean;
}

export async function login(username: string, password: string) {
  const resp = await fetch(`${API}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  const data = await resp.json();
  if (!resp.ok) throw new Error(data.error || 'Login failed');
  setTokens(data.access_token, data.refresh_token);
  localStorage.setItem('user', JSON.stringify(data.user));
  return data.user as UserInfo;
}

export async function logout() {
  try {
    await apiFetch('/auth/logout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
  } catch { /* ignore */ }
  clearTokens();
}

export async function listFiles(parentId: string = 'root'): Promise<FileItem[]> {
  const path = parentId === 'root' ? '/files' : `/files/${parentId}/children`;
  const resp = await apiFetch(path);
  if (!resp.ok) throw new Error('Failed to list files');
  return resp.json();
}

export async function createFolder(parentId: string, name: string): Promise<FileItem> {
  const resp = await apiFetch('/files', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ parent_id: parentId, name }),
  });
  const data = await resp.json();
  if (!resp.ok) throw new Error(data.error || 'Failed to create folder');
  return data;
}

export async function uploadFile(parentId: string, file: File, onProgress?: (pct: number) => void): Promise<FileItem> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${API}/files`);
    if (accessToken) xhr.setRequestHeader('Authorization', `Bearer ${accessToken}`);

    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
      };
    }

    xhr.onload = () => {
      const data = JSON.parse(xhr.responseText);
      if (xhr.status >= 200 && xhr.status < 300) resolve(data);
      else reject(new Error(data.error || 'Upload failed'));
    };
    xhr.onerror = () => reject(new Error('Upload failed'));

    const form = new FormData();
    form.append('file', file);
    form.append('parent_id', parentId);
    xhr.send(form);
  });
}

export async function deleteFile(id: string): Promise<void> {
  const resp = await apiFetch(`/files/${id}`, { method: 'DELETE' });
  if (!resp.ok) throw new Error('Delete failed');
}

export async function renameFile(id: string, name: string): Promise<FileItem> {
  const resp = await apiFetch(`/files/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  return resp.json();
}

export async function listPhotos(limit = 50, offset = 0): Promise<{ photos: PhotoItem[]; total: number }> {
  const resp = await apiFetch(`/photos?limit=${limit}&offset=${offset}`);
  return resp.json();
}

export async function listTimeline(): Promise<TimelineGroup[]> {
  const resp = await apiFetch('/photos/timeline');
  return resp.json();
}

// --- Shares ---

export interface ShareInfo {
  id: string;
  file_id: string;
  file_name: string;
  token: string;
  url: string;
  has_password: boolean;
  expires_at?: string;
  max_downloads?: number;
  download_count: number;
  created_at: string;
}

export async function createShare(fileId: string, password?: string, expiresIn?: number, maxDownloads?: number): Promise<ShareInfo> {
  const body: any = { file_id: fileId };
  if (password) body.password = password;
  if (expiresIn) body.expires_in = expiresIn;
  if (maxDownloads) body.max_downloads = maxDownloads;

  const resp = await apiFetch('/shares', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const data = await resp.json();
  if (!resp.ok) throw new Error(data.error || 'Failed to create share');
  return data;
}

export async function listShares(): Promise<ShareInfo[]> {
  const resp = await apiFetch('/shares');
  return resp.json();
}

export async function revokeShare(id: string): Promise<void> {
  await apiFetch(`/shares/${id}`, { method: 'DELETE' });
}

// --- Tags ---

export interface Tag {
  id: string;
  name: string;
  color: string;
  count?: number;
}

export async function listTags(): Promise<Tag[]> {
  const resp = await apiFetch('/tags');
  return resp.json();
}

export async function createTag(name: string, color?: string): Promise<Tag> {
  const resp = await apiFetch('/tags', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, color }),
  });
  return resp.json();
}

export async function deleteTag(id: string): Promise<void> {
  await apiFetch(`/tags/${id}`, { method: 'DELETE' });
}

export async function getFileTags(fileId: string): Promise<Tag[]> {
  const resp = await apiFetch(`/files/${fileId}/tags`);
  return resp.json();
}

export async function tagFile(fileId: string, tagIdOrName: string): Promise<Tag[]> {
  const body: any = {};
  // If it looks like a UUID, use tag_id; otherwise treat as name (auto-create)
  if (tagIdOrName.includes('-') && tagIdOrName.length > 30) {
    body.tag_id = tagIdOrName;
  } else {
    body.name = tagIdOrName;
  }
  const resp = await apiFetch(`/files/${fileId}/tags`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  return resp.json();
}

export async function untagFile(fileId: string, tagId: string): Promise<Tag[]> {
  const resp = await apiFetch(`/files/${fileId}/tags/${tagId}`, { method: 'DELETE' });
  return resp.json();
}

export async function listFilesByTag(tagId: string): Promise<FileItem[]> {
  const resp = await apiFetch(`/tags/${tagId}/files`);
  return resp.json();
}
