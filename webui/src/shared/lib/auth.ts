export function saveAuth(username: string, password: string) {
  localStorage.setItem("auth", btoa(`${username}:${password}`));
  localStorage.setItem("auth_type", "basic");
}

export function saveTokenAuth(token: string, username: string) {
  localStorage.setItem("auth", token);
  localStorage.setItem("auth_type", "bearer");
  localStorage.setItem("username", username);
}

export function getAuthHeader(): Record<string, string> {
  const auth = localStorage.getItem("auth");
  if (!auth) return {};
  const type = localStorage.getItem("auth_type");
  if (type === "bearer") {
    return { Authorization: `Bearer ${auth}` };
  }
  return { Authorization: `Basic ${auth}` };
}

export function isAuthenticated(): boolean {
  return Boolean(localStorage.getItem("auth"));
}

export function logout() {
  localStorage.removeItem("auth");
  localStorage.removeItem("auth_type");
  localStorage.removeItem("username");
}
