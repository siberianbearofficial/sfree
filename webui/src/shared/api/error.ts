import {addToast} from "@heroui/toast";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function throwIfNotOk(res: Response, fallback: string) {
  if (res.ok) return;
  let message = fallback;
  try {
    const body = await res.json();
    if (body.error) message = body.error;
    else if (body.message) message = body.message;
  } catch {
    message = fallback;
  }
  throw new ApiError(res.status, message);
}

export function showErrorToast(err: unknown) {
  if (err instanceof ApiError) {
    if (err.status === 401 || err.status === 403) {
      addToast({title: "Session expired", description: "Please log in again.", color: "danger", timeout: 6000});
      return;
    }
    if (err.status >= 500) {
      addToast({title: "Something went wrong", description: "The server returned an error. Try again later.", color: "danger", timeout: 6000});
      return;
    }
    addToast({title: "Action failed", description: err.message, color: "danger", timeout: 6000});
    return;
  }
  if (err instanceof TypeError) {
    addToast({title: "Connection failed", description: "Could not reach the server. Check your network and try again.", color: "danger", timeout: 6000});
    return;
  }
  addToast({title: "Action failed", description: "Something unexpected happened. Try again.", color: "danger", timeout: 6000});
}
