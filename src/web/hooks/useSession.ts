import { useEffect, useState } from 'react';

type Session =
  | { status: 'loading' | 'signed-out' | 'denied' | 'error' }
  | { status: 'signed-in'; email: string };

export function useSession() {
  const [session, setSession] = useState<Session>({ status: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();

    async function loadSession() {
      setSession({ status: 'loading' });

      if (window.location.pathname === '/auth/callback') {
        setSession({ status: 'denied' });
        return;
      }

      try {
        const response = await fetch('/api/session', {
          credentials: 'same-origin',
          signal: controller.signal,
        });

        if (response.status === 401) {
          setSession({ status: 'signed-out' });
        } else if (response.status === 403) {
          setSession({ status: 'denied' });
        } else if (response.ok) {
          const body = await response.json();
          if (typeof body.email !== 'string' || body.email === '') {
            throw new Error('Invalid session response');
          }
          setSession({ status: 'signed-in', email: body.email });
        } else {
          setSession({ status: 'error' });
        }
      } catch {
        if (!controller.signal.aborted) {
          setSession({ status: 'error' });
        }
      }
    }

    void loadSession();
    return () => controller.abort();
  }, [attempt]);

  return { session, retry: () => setAttempt((value) => value + 1) };
}
