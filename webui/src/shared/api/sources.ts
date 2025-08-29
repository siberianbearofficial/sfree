const API_BASE = "https://s3aas-api.dev.nachert.art/api/v1";

export type Source = {id: string; name: string; type: string; key: string; created_at: string};

function authHeader(): Record<string, string> {
  const auth = localStorage.getItem("auth");
  return auth ? {Authorization: `Basic ${auth}`} : {};
}

export async function listSources(): Promise<Source[]> {
  const res = await fetch(`${API_BASE}/sources`, {
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to list sources");
  return res.json();
}

export async function createSource(name: string, key: string): Promise<Source> {
  const res = await fetch(`${API_BASE}/sources/gdrive`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...authHeader(),
    },
    body: JSON.stringify({name, key}),
  });
  if (!res.ok) throw new Error("failed to create source");
  return res.json();
}
