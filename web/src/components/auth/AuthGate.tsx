import type { ReactNode } from 'react';
import { useAuth } from '@/lib/auth';
import { ErrorState } from '@/components/Page';
import { ProfilePicker } from './ProfilePicker';
import { Setup } from './Setup';
import { SignIn } from './SignIn';

/** Shows first-run setup, sign-in or the profile picker until a profile is in use, then the app. */
export function AuthGate({ children }: { children: ReactNode }) {
  const { data: status, error } = useAuth();
  if (error) return <ErrorState error={error} />;
  if (!status) return null;
  if (status.setup_needed) return <Setup />;
  if (status.profile) return <>{children}</>;
  if (status.signed_in || status.bypass) return <ProfilePicker status={status} />;
  return <SignIn />;
}
