// Photos travel with auth: <img src> cannot carry a bearer header, so the bytes
// are fetched and shown through an object URL, cached per path for the life of
// the page. shrink() makes a phone photo small before it leaves the phone.
// The same helpers serve a tool's photo (/tools/{id}/photo) and a project's
// pictures (/photos/{id}).
import { API, getToken } from "./api";

const cache = new Map<string, Promise<string>>();
const auth = () => ({ Authorization: "Bearer " + (getToken() || "") });

export function photoURL(path: string, bust = 0): Promise<string> {
  const key = `${path}#${bust}`;
  let p = cache.get(key);
  if (!p) {
    p = fetch(API + path, { headers: auth() })
      .then((r) => (r.ok ? r.blob() : Promise.reject(r.status)))
      .then((b) => URL.createObjectURL(b));
    cache.set(key, p);
  }
  return p;
}

export function forgetPhoto(path: string) {
  for (const k of [...cache.keys()]) if (k.startsWith(path + "#")) cache.delete(k);
}

// shrink: longest side 1000 px, JPEG 0.82 — about 100–200 KB from a 4 MB shot.
export async function shrink(file: File): Promise<Blob> {
  const url = URL.createObjectURL(file);
  try {
    const img = await new Promise<HTMLImageElement>((ok, no) => { const i = new Image(); i.onload = () => ok(i); i.onerror = no; i.src = url; });
    const k = Math.min(1, 1000 / Math.max(img.width, img.height));
    const c = document.createElement("canvas");
    c.width = Math.round(img.width * k); c.height = Math.round(img.height * k);
    c.getContext("2d")!.drawImage(img, 0, 0, c.width, c.height);
    return await new Promise<Blob>((ok, no) => c.toBlob((b) => (b ? ok(b) : no(new Error("toBlob"))), "image/jpeg", 0.82));
  } finally { URL.revokeObjectURL(url); }
}

// uploadPhoto: PUT replaces the one photo at `path` (a tool); POST adds one
// more to a collection (a project) and returns what the server said.
export async function uploadPhoto(path: string, file: File, method: "PUT" | "POST" = "PUT"): Promise<any> {
  const blob = await shrink(file);
  const r = await fetch(API + path, { method, headers: { ...auth(), "Content-Type": "image/jpeg" }, body: blob });
  if (!r.ok) throw new Error("upload " + r.status);
  forgetPhoto(path);
  return r.status === 204 ? undefined : r.json();
}
