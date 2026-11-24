import { useEffect, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, type AuthStatus } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { autoProfile, freshStart } from '@/lib/launch';
import { ErrorState } from '@/components/Page';
import { DatabaseOutage, useDatabaseOutage } from './DatabaseOutage';
import { ProfilePicker } from './ProfilePicker';
import { DatabaseSetup } from './DatabaseSetup';
import { Setup } from './Setup';
import { SignIn } from './SignIn';
import { AcceptInvite } from './AcceptInvite';

/** Shows first-run setup, sign-in or the profile picker until a profile is in use, then the app. */
export function AuthGate({ children }: { children: ReactNode }) {
  const { data: status, error } = useAuth();
  const launching = useFreshStart(status);
  const databaseOffline = useDatabaseOutage(error);
  if (databaseOffline && (!status || !status.profile || launching)) return <DatabaseOutage fullPage />;
  if (error && !databaseOffline) return <ErrorState error={error} />;
  if (!status || launching) return null;
  if (status.database_needed) return <DatabaseSetup />;
  if (status.setup_needed) return <Setup />;
  // An invite link from the admin, for a friend to choose their password.
  const invite = window.location.pathname.match(/^\/invite\/([^/]+)$/);
  if (invite) return <AcceptInvite token={decodeURIComponent(invite[1])} />;
  if (status.profile) return <>{databaseOffline && <DatabaseOutage />}{children}</>;
  if (status.signed_in || status.bypass) return <ProfilePicker status={status} />;
  return <SignIn />;
}

/**
 * On a fresh start, opens "Who's watching?" (leaving the profile the last visit used), or signs in
 * as this browser's automatic profile. True while that is being settled.
 */
function useFreshStart(status: AuthStatus | undefined) {
  const qc = useQueryClient();
  const [settling, setSettling] = useState(() => freshStart());
  useEffect(() => {
    if (!settling || !status) return;
    const auto = autoProfile();
    const done = () => void qc.invalidateQueries({ queryKey: ['auth'] }).finally(() => setSettling(false));
    if (status.setup_needed || !(status.signed_in || status.bypass)) {
      setSettling(false);
    } else if (status.profile) {
      if (status.profile.id === auto) setSettling(false);
      else void api.leaveProfile().catch(() => undefined).finally(done);
    } else if (auto) {
      // A profile with a PIN (or one that's gone) falls back to the picker.
      void api.chooseProfile(auto).catch(() => undefined).finally(done);
    } else {
      setSettling(false);
    }
  }, [settling, status, qc]);
  return settling;
}
