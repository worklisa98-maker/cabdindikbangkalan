export const TOKEN_KEY = "cabdin_token";
export const AUTH_MESSAGE_KEY = "cabdin_auth_message";

export const getToken = () => {
  if (typeof window === "undefined") return null;
  const localToken = window.localStorage.getItem(TOKEN_KEY);
  if (localToken) return localToken;
  const match = document.cookie.match(
    new RegExp(`(?:^|; )${TOKEN_KEY}=([^;]*)`)
  );
  return match ? decodeURIComponent(match[1]) : null;
};

export const setToken = (token: string) => {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(TOKEN_KEY, token);
  document.cookie = `${TOKEN_KEY}=${encodeURIComponent(
    token
  )}; path=/; SameSite=Lax`;
};

export const clearToken = () => {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(TOKEN_KEY);
  document.cookie = `${TOKEN_KEY}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Lax`;
};
