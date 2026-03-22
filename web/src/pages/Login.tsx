import { createSignal } from 'solid-js';
import { login } from '../stores/auth';

export default function Login() {
  const [username, setUsername] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(username(), password());
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div class="login-container">
      <div class="login-card">
        <h1>SelfShare</h1>
        <p class="subtitle">Sign in to your file server</p>
        <form onSubmit={handleSubmit}>
          <div class="field">
            <label>Username</label>
            <input
              type="text"
              value={username()}
              onInput={(e) => setUsername(e.currentTarget.value)}
              required
              autofocus
              autocomplete="username"
            />
          </div>
          <div class="field">
            <label>Password</label>
            <input
              type="password"
              value={password()}
              onInput={(e) => setPassword(e.currentTarget.value)}
              required
              autocomplete="current-password"
            />
          </div>
          <button class="btn-primary full-width" type="submit" disabled={loading()}>
            {loading() ? 'Signing in...' : 'Sign In'}
          </button>
          {error() && <div class="error-text">{error()}</div>}
        </form>
      </div>
    </div>
  );
}
