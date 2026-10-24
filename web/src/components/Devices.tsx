import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Globe, Loader2, MonitorSmartphone, Tv } from 'lucide-react';
import { api, type Device } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Button } from '@/components/ui/button';
import { ErrorNotice, Page, PageHeader } from '@/components/Page';

/** Link a Device: enter the code a TV shows to sign it in, as plex.tv/link does. */
export function LinkDevicePage() {
  const [code, setCode] = useState('');
  const link = useMutation({ mutationFn: () => api.approveLink(code) });
  return (
    <Page className="pb-16">
      <PageHeader title="Link a Device" subtitle="Signing in an Arc app on a TV or another device? Enter the code it shows." />
      <div className="max-w-md px-6 md:px-10">
        {link.isSuccess ? (
          <div className="rounded-2xl bg-surface p-6 ring-1 ring-border">
            <p className="flex items-center gap-2 font-semibold">
              <CheckCircle2 className="size-5 text-accent" /> {link.data.device} is signed in
            </p>
            <p className="mt-1 text-sm text-muted">It can be signed out again in Settings &gt; Devices.</p>
            <Button
              variant="secondary"
              className="mt-4"
              onClick={() => {
                setCode('');
                link.reset();
              }}
            >
              Link Another
            </Button>
          </div>
        ) : (
          <form
            className="rounded-2xl bg-surface p-6 ring-1 ring-border"
            onSubmit={(e) => {
              e.preventDefault();
              link.mutate();
            }}
          >
            <input
              autoFocus
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 4))}
              placeholder="ABCD"
              aria-label="Code"
              autoComplete="off"
              spellCheck={false}
              className="h-16 w-full rounded-xl bg-surface-2/80 text-center font-mono text-3xl tracking-[0.5em] ring-1 ring-border outline-none placeholder:text-subtle focus:ring-2 focus:ring-accent"
            />
            {link.error && <p className="mt-3 text-sm text-warning">{link.error.message}</p>}
            <Button type="submit" variant="primary" size="lg" className="mt-4 w-full" disabled={code.length < 4 || link.isPending}>
              {link.isPending && <Loader2 className="animate-spin" />} Link
            </Button>
          </form>
        )}
      </div>
    </Page>
  );
}

/** Settings: every signed-in browser and app, to sign out ones that shouldn't be. */
export function DevicesList() {
  const devices = useQuery({ queryKey: ['devices'], queryFn: api.devices });
  const { data } = devices;
  const [all, setAll] = useState(false);
  const feedback = devices.error && <div className="m-4 space-y-2">
    {data && <p className="text-xs text-muted">Showing the last loaded devices. They may be out of date.</p>}
    <ErrorNotice error={devices.error} onRetry={() => void devices.refetch()} busy={devices.isFetching} />
  </div>;
  if (!data) return feedback || <div role="status" className="m-4 h-12 animate-pulse rounded-xl bg-surface-2"><span className="sr-only">Loading devices…</span></div>;
  return (
    <>
      {feedback}
      {data.length === 0 && <p className="px-4 py-3 text-sm text-muted">No devices are signed in.</p>}
      {(all ? data : data.slice(0, 8)).map(d => <DeviceRow key={d.id} device={d} />)}
      {!all && data.length > 8 && (
        <button type="button" onClick={() => setAll(true)} className="w-full px-4 py-3 text-left text-sm text-accent">
          Show all {data.length}
        </button>
      )}
    </>
  );
}

// Each row owns its request/error so one device's retry cannot clear another's failure.
function DeviceRow({ device: d }: { device: Device }) {
  const qc = useQueryClient();
  const signOut = useMutation({
    mutationFn: () => api.signOutDevice(d.id),
    onSuccess: () => {
      // Remove only after the server confirms revocation, even if the following refresh fails.
      qc.setQueryData<Device[]>(['devices'], list => list?.filter(device => device.id !== d.id));
      return qc.invalidateQueries({ queryKey: ['devices'] });
    },
  });
  const name = d.device || browserName(d.user_agent);
  return (
    <div role="group" aria-label={`Device: ${name}`} className="flex flex-wrap items-center gap-3 px-4 py-2.5">
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-surface-2 text-muted [&_svg]:size-4">
        {d.device ? <Tv /> : d.via === 'local' ? <Globe /> : <MonitorSmartphone />}
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">
          {name}{d.this_device && <span className="font-normal text-muted"> · this browser</span>}
        </p>
        <p className="truncate text-xs text-subtle">
          {[d.profile || (d.via === 'friend' ? 'Friend' : 'Choosing a profile'), viaLabel[d.via], `used ${fmt.ago(d.last_seen_at)}`].filter(Boolean).join(' · ')}
        </p>
      </div>
      {!d.this_device && <Button variant="ghost" size="sm" disabled={signOut.isPending} onClick={() => signOut.mutate()}>
        {signOut.isPending && <Loader2 className="animate-spin" />}{signOut.error ? 'Retry' : 'Sign Out'}
      </Button>}
      {signOut.error && <div className="w-full"><ErrorNotice error={signOut.error} /></div>}
    </div>
  );
}

const viaLabel: Record<Device['via'], string> = { password: 'with the password', local: 'at home without the password', friend: 'friend' };

/** "Chrome on Windows" from a user agent. */
function browserName(ua: string) {
  const browser = /Edg\//.test(ua) ? 'Edge' : /Firefox\//.test(ua) ? 'Firefox' : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : /CrKey/.test(ua) ? 'Chromecast' : '';
  const os = /Windows/.test(ua) ? 'Windows' : /iPhone|iPad/.test(ua) ? 'iOS' : /Mac OS/.test(ua) ? 'Mac' : /Android/.test(ua) ? 'Android' : /Linux/.test(ua) ? 'Linux' : '';
  return browser && os ? `${browser} on ${os}` : browser || os || 'A browser or app';
}
