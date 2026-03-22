import { createSignal, onMount, Show, For } from 'solid-js';
import { type PhotoItem, type FileItem, listPhotos, thumbUrl } from '../api/client';

export default function PhotoGallery(props: { onViewPhoto: (photo: FileItem, all: FileItem[]) => void }) {
  const [photos, setPhotos] = createSignal<PhotoItem[]>([]);
  const [total, setTotal] = createSignal(0);
  const [loading, setLoading] = createSignal(false);
  const [offset, setOffset] = createSignal(0);
  const limit = 60;

  onMount(() => load(0));

  async function load(off: number) {
    setLoading(true);
    try {
      const data = await listPhotos(limit, off);
      if (off === 0) {
        setPhotos(data.photos || []);
      } else {
        setPhotos((prev) => [...prev, ...(data.photos || [])]);
      }
      setTotal(data.total);
      setOffset(off);
    } catch (e) {
      console.error('Failed to load photos:', e);
    } finally {
      setLoading(false);
    }
  }

  function groupByDate(items: PhotoItem[]): { label: string; photos: PhotoItem[] }[] {
    const groups: Record<string, PhotoItem[]> = {};
    for (const p of items) {
      const date = p.taken_at || p.created_at;
      const d = new Date(date);
      const key = d.toLocaleDateString(undefined, { year: 'numeric', month: 'long' });
      if (!groups[key]) groups[key] = [];
      groups[key].push(p);
    }
    return Object.entries(groups).map(([label, photos]) => ({ label, photos }));
  }

  function handleClick(photo: PhotoItem) {
    const allAsFile = photos().map((p) => p as unknown as FileItem);
    props.onViewPhoto(photo as unknown as FileItem, allAsFile);
  }

  return (
    <div class="gallery-container">
      <Show when={photos().length === 0 && !loading()}>
        <div class="empty">No photos yet. Upload some images to see them here.</div>
      </Show>

      <For each={groupByDate(photos())}>
        {(group) => (
          <div class="gallery-group">
            <h3 class="gallery-date">{group.label}</h3>
            <div class="gallery-grid">
              <For each={group.photos}>
                {(photo) => (
                  <div class="gallery-thumb" onClick={() => handleClick(photo)}>
                    <img
                      src={thumbUrl(photo.id, 'sm')}
                      alt={photo.name}
                      loading="lazy"
                    />
                    <Show when={photo.mime_type?.startsWith('video/')}>
                      <div class="video-badge">{'\u25B6'}</div>
                    </Show>
                  </div>
                )}
              </For>
            </div>
          </div>
        )}
      </For>

      <Show when={photos().length < total()}>
        <div class="load-more">
          <button class="btn-secondary" onClick={() => load(offset() + limit)} disabled={loading()}>
            {loading() ? 'Loading...' : `Load More (${photos().length} / ${total()})`}
          </button>
        </div>
      </Show>
    </div>
  );
}
