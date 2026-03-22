import { createSignal } from 'solid-js';
import { type UserInfo, login as apiLogin, logout as apiLogout, setAuthFailureHandler } from '../api/client';

const stored = localStorage.getItem('user');
const [user, setUser] = createSignal<UserInfo | null>(stored ? JSON.parse(stored) : null);
const [isLoggedIn, setIsLoggedIn] = createSignal(!!localStorage.getItem('access_token'));

setAuthFailureHandler(() => {
  setUser(null);
  setIsLoggedIn(false);
});

export { user, isLoggedIn };

export async function login(username: string, password: string) {
  const u = await apiLogin(username, password);
  setUser(u);
  setIsLoggedIn(true);
  return u;
}

export async function logout() {
  await apiLogout();
  setUser(null);
  setIsLoggedIn(false);
}
