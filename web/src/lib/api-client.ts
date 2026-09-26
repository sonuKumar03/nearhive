export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export async function fetchApi<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = typeof window !== 'undefined' ? localStorage.getItem('nearhive_token') : null;

  const headers = new Headers(options.headers || {});
  headers.set('Content-Type', 'application/json');
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const res = await fetch(path, {
    ...options,
    headers,
  });

  if (!res.ok) {
    let errorData: Record<string, any> = {};
    try {
      errorData = await res.json();
    } catch {}

    if (res.status === 401 && typeof window !== 'undefined') {
      localStorage.removeItem('nearhive_token');
    }

    const errorMessage = errorData.error || errorData.message || res.statusText || 'An unexpected error occurred';
    const errorCode = errorData.code || 'API_ERROR';
    throw new ApiError(res.status, errorCode, errorMessage);
  }

  if (res.status === 204) {
    return {} as T;
  }

  const text = await res.text();
  return (text ? JSON.parse(text) : {}) as T;
}
