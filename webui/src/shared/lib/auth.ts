export function saveAuth(username: string, password: string) {
  localStorage.setItem("auth", btoa(`${username}:${password}`));
  localStorage.setItem("auth_type", "basic");
}

export function saveTokenAuth(token: string, username: string) {
  localStorage.setItem("auth", token);
  localStorage.setItem("auth_type", "bearer");
  localStorage.setItem("username", username);
}

export function saveCookieAuth(username: string) {
  localStorage.setItem("auth_type", "cookie");
  localStorage.setItem("username", username);
}

export function getAuthHeader(): Record<string, string> {
  const type = localStorage.getItem("auth_type");
  if (type === "cookie") {
    return {};
  }
  const auth = localStorage.getItem("auth");
  if (!auth) return {};
  if (type === "bearer") {
    return { Authorization: `Bearer ${auth}` };
  }
  return { Authorization: `Basic ${auth}` };
}

export function isAuthenticated(): boolean {
  const type = localStorage.getItem("auth_type");
  if (type === "cookie") return true;
  return Boolean(localStorage.getItem("auth"));
}

export function logout() {
  localStorage.removeItem("auth");
  localStorage.removeItem("auth_type");
  localStorage.removeItem("username");
  // Clear auth_token cookie by setting it expired.
  document.cookie = "auth_token=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Secure; SameSite=Lax";
}
