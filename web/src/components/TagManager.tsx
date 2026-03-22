import { createSignal, onMount, For, Show } from 'solid-js';
import {
  type Tag, type FileItem,
  listTags, createTag, deleteTag, getFileTags, tagFile, untagFile, listFilesByTag,
} from '../api/client';

// --- Tag badge component ---
export function TagBadge(props: { tag: Tag; onRemove?: () => void }) {
  return (
    <span class="tag-badge" style={{ "border-color": props.tag.color, color: props.tag.color }}>
      {props.tag.name}
      {props.onRemove && (
        <button class="tag-remove" onClick={(e) => { e.stopPropagation(); props.onRemove!(); }}>&times;</button>
      )}
    </span>
  );
}

// --- File tag editor (shown in a popover or inline) ---
export function FileTagEditor(props: { file: FileItem; onClose: () => void }) {
  const [fileTags, setFileTags] = createSignal<Tag[]>([]);
  const [allTags, setAllTags] = createSignal<Tag[]>([]);
  const [newTag, setNewTag] = createSignal('');

  onMount(async () => {
    const [ft, at] = await Promise.all([getFileTags(props.file.id), listTags()]);
    setFileTags(ft);
    setAllTags(at);
  });

  async function addTag(nameOrId: string) {
    const tags = await tagFile(props.file.id, nameOrId);
    setFileTags(tags);
    setAllTags(await listTags());
    setNewTag('');
  }

  async function removeTag(tagId: string) {
    const tags = await untagFile(props.file.id, tagId);
    setFileTags(tags);
  }

  function unusedTags() {
    const used = new Set(fileTags().map((t) => t.id));
    return allTags().filter((t) => !used.has(t.id));
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter' && newTag().trim()) {
      addTag(newTag().trim());
    }
  }

  return (
    <div class="tag-editor" onClick={(e) => e.stopPropagation()}>
      <div class="tag-editor-header">
        <strong>Tags for {props.file.name}</strong>
        <button class="tag-close" onClick={props.onClose}>&times;</button>
      </div>

      <div class="tag-list">
        <For each={fileTags()}>
          {(tag) => <TagBadge tag={tag} onRemove={() => removeTag(tag.id)} />}
        </For>
        <Show when={fileTags().length === 0}>
          <span class="tag-empty">No tags</span>
        </Show>
      </div>

      <div class="tag-input-row">
        <input
          type="text"
          placeholder="Add tag..."
          value={newTag()}
          onInput={(e) => setNewTag(e.currentTarget.value)}
          onKeyDown={handleKeyDown}
        />
        <button class="btn-secondary" onClick={() => newTag().trim() && addTag(newTag().trim())}>Add</button>
      </div>

      <Show when={unusedTags().length > 0}>
        <div class="tag-suggestions">
          <For each={unusedTags()}>
            {(tag) => (
              <button class="tag-suggestion" style={{ "border-color": tag.color, color: tag.color }} onClick={() => addTag(tag.id)}>
                + {tag.name}
              </button>
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}

// --- Tag sidebar for filtering ---
export function TagSidebar(props: { onSelectTag: (tag: Tag | null) => void; selectedTagId: string | null }) {
  const [tags, setTags] = createSignal<Tag[]>([]);
  const [newName, setNewName] = createSignal('');

  onMount(loadTags);

  async function loadTags() {
    setTags(await listTags());
  }

  async function handleCreate() {
    if (!newName().trim()) return;
    await createTag(newName().trim());
    setNewName('');
    await loadTags();
  }

  async function handleDelete(id: string) {
    if (!confirm('Delete this tag?')) return;
    await deleteTag(id);
    if (props.selectedTagId === id) props.onSelectTag(null);
    await loadTags();
  }

  return (
    <div class="tag-sidebar">
      <div class="tag-sidebar-header">
        <strong>Tags</strong>
      </div>

      <button
        class={props.selectedTagId === null ? 'tag-filter active' : 'tag-filter'}
        onClick={() => props.onSelectTag(null)}
      >
        All Files
      </button>

      <For each={tags()}>
        {(tag) => (
          <div class="tag-filter-row">
            <button
              class={props.selectedTagId === tag.id ? 'tag-filter active' : 'tag-filter'}
              onClick={() => props.onSelectTag(tag)}
            >
              <span class="tag-dot" style={{ background: tag.color }} />
              {tag.name}
              <span class="tag-count">{tag.count}</span>
            </button>
            <button class="tag-delete-btn" onClick={() => handleDelete(tag.id)}>&times;</button>
          </div>
        )}
      </For>

      <div class="tag-create-row">
        <input
          type="text"
          placeholder="New tag..."
          value={newName()}
          onInput={(e) => setNewName(e.currentTarget.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
        />
      </div>
    </div>
  );
}
