import { For } from 'solid-js';
import { breadcrumbs, navigateToBreadcrumb } from '../stores/files';

export default function Breadcrumbs() {
  return (
    <div class="breadcrumbs">
      <For each={breadcrumbs()}>
        {(crumb, index) => (
          <>
            {index() > 0 && <span class="sep">/</span>}
            {index() === breadcrumbs().length - 1 ? (
              <strong>{crumb.name}</strong>
            ) : (
              <a onClick={() => navigateToBreadcrumb(index())}>{crumb.name}</a>
            )}
          </>
        )}
      </For>
    </div>
  );
}
