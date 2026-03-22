import { render } from 'solid-js/web';
import App from './App';
import './styles.css';
import { loadFiles } from './stores/files';

// Load files on startup if logged in
if (localStorage.getItem('access_token')) {
  loadFiles();
}

render(() => <App />, document.getElementById('root')!);
