import { user, logout } from '../stores/auth';

export default function Header(props: { activeTab: string; onTabChange: (tab: string) => void }) {
  return (
    <header class="header">
      <div class="header-left">
        <h1 class="logo">SelfShare</h1>
        <nav class="tabs">
          <button
            class={props.activeTab === 'files' ? 'tab active' : 'tab'}
            onClick={() => props.onTabChange('files')}
          >
            Files
          </button>
          <button
            class={props.activeTab === 'photos' ? 'tab active' : 'tab'}
            onClick={() => props.onTabChange('photos')}
          >
            Photos
          </button>
        </nav>
      </div>
      <div class="header-right">
        <span class="user-name">{user()?.display_name || user()?.username}</span>
        <button class="btn-ghost" onClick={logout}>Sign Out</button>
      </div>
    </header>
  );
}
