export function saveAuth(username: string, password: string) {
  localStorage.setItem("auth", btoa(`${username}:${password}`));
}
