import { useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, UNAUTHORIZED_EVENT } from './api';

/** Who is signed in, and as which profile. Refetched whenever the server says a session is gone. */
export function useAuth() {
  const qc = useQueryClient();
  const query = useQuery({ queryKey: ['auth'], queryFn: api.authStatus, staleTime: Infinity });
  useEffect(() => {
    const onUnauthorized = () => qc.invalidateQueries({ queryKey: ['auth'] });
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
  }, [qc]);
  return query;
}

/** The current profile is the admin. */
export function useIsAdmin() {
  return !!useAuth().data?.profile?.is_admin;
}
