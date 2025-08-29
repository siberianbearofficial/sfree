const API_BASE = "https://s3aas-api.dev.nachert.art/api/v1";

export type Bucket = {id: string; key: string; created_at: string};

function authHeader(): Record<string, string> {
  const auth = localStorage.getItem("auth");
  return auth ? {Authorization: `Basic ${auth}`} : {};
}

export async function listBuckets(): Promise<Bucket[]> {
  const res = await fetch(`${API_BASE}/buckets`, {
    headers: authHeader(),
  });
  if (!res.ok) throw new Error("failed to list buckets");
  return res.json();
}

export async function createBucket(key: string): Promise<{key: string; access_key: string; access_secret: string; created_at: string}> {
  const res = await fetch(`${API_BASE}/buckets`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...authHeader(),
    },
    body: JSON.stringify({key}),
  });
  if (!res.ok) throw new Error("failed to create bucket");
  return res.json();
}
