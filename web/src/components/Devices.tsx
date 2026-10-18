import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Globe, Loader2, MonitorSmartphone, Tv } from 'lucide-react';
import { api, type Device } from '@/lib/api';
import * as fmt from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Page, PageHeader } from '@/components/Page';

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
  const qc = useQueryClient();
  const { data } = useQuery({ queryKey: ['devices'], queryFn: api.devices });
  const [all, setAll] = useState(false);
  const signOut = useMutation({
    mutationFn: (d: Device) => api.signOutDevice(d.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['devices'] }),
  });
  if (!data) return <div className="m-4 h-12 animate-pulse rounded-xl bg-surface-2" />;
  return (
    <>
      {(all ? data : data.slice(0, 8)).map((d) => (
        <div key={d.id} className="flex items-center gap-3 px-4 py-2.5">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-surface-2 text-muted [&_svg]:size-4">
            {d.device ? <Tv /> : d.via === 'local' ? <Globe /> : <MonitorSmartphone />}
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">
              {d.device || browserName(d.user_agent)}
              {d.this_device && <span className="font-normal text-muted"> · this browser</span>}
            </p>
            <p className="truncate text-xs text-subtle">
              {[d.profile || (d.via === 'friend' ? 'Friend' : 'Choosing a profile'), viaLabel[d.via], `used ${fmt.ago(d.last_seen_at)}`].filter(Boolean).join(' · ')}
            </p>
          </div>
          {!d.this_device && (
            <Button variant="ghost" size="sm" disabled={signOut.isPending} onClick={() => signOut.mutate(d)}>
              Sign Out
            </Button>
          )}
        </div>
      ))}
      {!all && data.length > 8 && (
        <button type="button" onClick={() => setAll(true)} className="w-full px-4 py-3 text-left text-sm text-accent hover:underline">
          Show all {data.length}
        </button>
      )}
    </>
  );
}

const viaLabel: Record<Device['via'], string> = { password: 'with the password', local: 'at home without the password', friend: 'friend' };

/** "Chrome on Windows" from a user agent. */
function browserName(ua: string) {
  const browser = /Edg\//.test(ua) ? 'Edge' : /Firefox\//.test(ua) ? 'Firefox' : /Chrome\//.test(ua) ? 'Chrome' : /Safari\//.test(ua) ? 'Safari' : /CrKey/.test(ua) ? 'Chromecast' : '';
  const os = /Windows/.test(ua) ? 'Windows' : /iPhone|iPad/.test(ua) ? 'iOS' : /Mac OS/.test(ua) ? 'Mac' : /Android/.test(ua) ? 'Android' : /Linux/.test(ua) ? 'Linux' : '';
  return browser && os ? `${browser} on ${os}` : browser || os || 'A browser or app';
}
