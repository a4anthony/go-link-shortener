import { useEffect, useState } from 'react';

export interface AsyncState<T> {
  data?: T;
  error?: string;
  loading: boolean;
  refetch: () => void;
}

/**
 * useAsync runs an async function on mount and whenever `deps` change, tracking
 * loading/error/data and exposing a manual refetch.
 */
export function useAsync<T>(fn: () => Promise<T>, deps: unknown[]): AsyncState<T> {
  const [state, setState] = useState<{ data?: T; error?: string; loading: boolean }>({
    loading: true,
  });
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let alive = true;
    setState((s) => ({ ...s, loading: true, error: undefined }));
    fn()
      .then((d) => alive && setState({ data: d, loading: false }))
      .catch((e: unknown) =>
        alive && setState({ error: e instanceof Error ? e.message : String(e), loading: false }),
      );
    return () => {
      alive = false;
    };
    // fn is intentionally excluded; callers control invalidation via deps.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, nonce]);

  return { ...state, refetch: () => setNonce((n) => n + 1) };
}
