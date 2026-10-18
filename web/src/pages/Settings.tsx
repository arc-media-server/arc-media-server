import { useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Copy, ExternalLink, Loader2, XCircle } from 'lucide-react';
import { api, type RemoteStatus, type ServerSettings, type Theme } from '@/lib/api';
import { useIsAdmin } from '@/lib/auth';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import { ErrorState, Page, PageHeader } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { ImportDialog } from '@/components/ImportDialog';
import { qualities, qualityName } from '@/lib/playback';
import { Webhooks } from '@/components/Webhooks';
import { OptimizedList } from '@/components/Optimize';
import { DevicesList } from '@/components/Devices';

type Patch = Parameters<typeof api.updateSettings>[0];

export function SettingsPage() {
  const admin = useIsAdmin();
  return (
    <Page className="pb-16">
      <PageHeader title="Settings" subtitle={admin ? 'Your profile, and everything about how this Arc server runs.' : 'How Arc works for your profile.'} />
      <div className="max-w-3xl space-y-10 px-6 md:px-10">
        <ProfileSettings />
        {admin && <ServerSettings />}
      </div>
    </Page>
  );
}

/** The current profile's own settings (each profile has its own, as in Plex). */
function ProfileSettings() {
  const prefs = usePrefs();
  const update = useUpdatePrefs();
  const p = prefs.data;
  if (!p) return <div className="h-48 animate-pulse rounded-2xl bg-surface" />;
  const busy = update.isPending;
  return (
    <>
      <Section title="Playback" hint="For your profile only.">
        <ToggleRow
          label="Auto-Play"
          description="Play the next episode after a short countdown."
          checked={p.auto_play !== false}
          busy={busy}
          onChange={(v) => update.mutate({ auto_play: v })}
        />
        <ToggleRow
          label="Skip intros automatically"
          description="Skip intros and recaps without pressing Skip."
          checked={!!p.auto_skip}
          busy={busy}
          onChange={(v) => update.mutate({ auto_skip: v })}
        />
        <ToggleRow
          label="Theme music"
          description="Play a show's or movie's theme quietly on its page, when there's one."
          checked={p.theme_music !== false}
          busy={busy}
          onChange={(v) => update.mutate({ theme_music: v })}
        />
      </Section>
      <Section title="Display">
        <ToggleRow
          label="Hide spoilers"
          description="Blur stills and hide descriptions of episodes you haven't seen."
          checked={!!p.hide_spoilers}
          busy={busy}
          onChange={(v) => update.mutate({ hide_spoilers: v })}
        />
        <Row label="Appearance" description="Automatic follows this device's light or dark setting.">
          <select
            value={p.theme ?? ''}
            disabled={busy}
            onChange={(e) => update.mutate({ theme: e.target.value as Theme })}
            className={`${inputClass} w-40 [&>option]:bg-surface`}
          >
            <option value="">Automatic</option>
            <option value="dark">Dark</option>
            <option value="light">Light</option>
          </select>
        </Row>
      </Section>
    </>
  );
}

/** The server's settings (admin). */
function ServerSettings() {
  const qc = useQueryClient();
  const settings = useQuery({ queryKey: ['settings'], queryFn: api.settings });
  const [importing, setImporting] = useState<'plex' | 'jellyfin' | null>(null);
  const update = useMutation({
    mutationFn: (patch: Patch) => api.updateSettings(patch),
    onSuccess: (s) => {
      qc.setQueryData(['settings'], s);
      qc.invalidateQueries({ queryKey: ['auth'] });
    },
  });

  if (settings.error) return <ErrorState error={settings.error} />;
  const s = settings.data;

  return (
    <>
      <h2 className="border-t border-border pt-8 text-xs font-semibold uppercase tracking-wider text-subtle">Server</h2>
      {!s ? (
        <div className="h-96 animate-pulse rounded-2xl bg-surface" />
      ) : (
        <>
          <Section title="Online metadata" hint="Arc identifies movies and shows and fetches their details and artwork from these services.">
            <KeyRow
              label="TMDB"
              description="Movies and shows."
              current={s.tmdb_api_key}
              fixed={s.from_env.tmdb_api_key}
              link="https://www.themoviedb.org/settings/api"
              onSave={(v) => update.mutateAsync({ tmdb_api_key: v })}
            />
            <KeyRow
              label="TheTVDB"
              description="Shows and their episode orders."
              current={s.tvdb_api_key}
              fixed={s.from_env.tvdb_api_key}
              link="https://thetvdb.com/api-information"
              onSave={(v) => update.mutateAsync({ tvdb_api_key: v })}
            />
            <Row label="Region" description="Age ratings (like 12A or PG-13) are shown for this country and used by parental controls.">
              <select
                value={s.region}
                onChange={(e) => update.mutate({ region: e.target.value })}
                disabled={update.isPending}
                className={`${inputClass} w-56 [&>option]:bg-surface`}
              >
                {regionOptions(s.region).map(([code, name]) => (
                  <option key={code} value={code}>
                    {name}
                  </option>
                ))}
              </select>
            </Row>
          </Section>

          <Section title="Sign-in" hint="Away from home, the password is always needed.">
            <ToggleRow
              label="Skip the password at home"
              description="On this computer and your home network, people just pick a profile. The admin profile still asks for the password or its PIN."
              checked={s.local_bypass}
              busy={update.isPending}
              onChange={(v) => update.mutate({ local_bypass: v })}
            />
            <PasswordRow />
          </Section>

          <Section title="Network">
            <ToggleRow
              label="Allow other devices"
              description="Let phones, TVs and other computers on your network use Arc. When off, only this computer can."
              checked={s.allow_network}
              busy={update.isPending}
              onChange={(v) => update.mutate({ allow_network: v })}
            />
            <PortRow settings={s} onSave={(port) => update.mutateAsync({ port })} />
            <HomeNetworksRow value={s.home_networks} onSave={(v) => update.mutateAsync({ home_networks: v })} />
          </Section>

          <Section title="Remote Access" hint="Watching away from home. The password is always needed there.">
            <RemoteRows settings={s} onSave={(p) => update.mutateAsync(p)} busy={update.isPending} />
            <Row label="Limit each remote stream" description="Videos are converted down to this when they're bigger, so they play smoothly on slower connections.">
              <select
                value={s.remote_kbps}
                disabled={update.isPending}
                onChange={(e) => update.mutate({ remote_kbps: Number(e.target.value) })}
                className={`${inputClass} w-48 [&>option]:bg-surface`}
              >
                <option value={0}>Original (no limit)</option>
                {qualities.map((q) => (
                  <option key={q} value={q}>
                    {qualityName(q)}
                  </option>
                ))}
              </select>
            </Row>
            <UploadRow kbps={s.upload_kbps} onSave={(kbps) => update.mutateAsync({ upload_kbps: kbps })} />
          </Section>

          <Section title="Devices" hint="Browsers and apps signed in to Arc. Sign out any you don't recognise; changing the password signs out everything that used it.">
            <DevicesList />
          </Section>

          <Section title="Optimized Versions" hint="Easy-to-play copies made ahead of time, played instead of converting live when a player needs them.">
            <OptimizedList />
          </Section>

          <Section title="Media tools" hint="Arc uses ffprobe (part of FFmpeg) to read what's inside your video files, and FFmpeg to convert them for players that need it.">
            <FFprobeRow settings={s} onSave={(path) => update.mutateAsync({ ffprobe_path: path })} />
          </Section>

          <Section title="Moving in" hint="Coming from another media server? Bring everyone's watch history with you.">
            <Row label="Plex" description="Watched titles, resume points, ratings and Plex's watch history, for everyone on the server.">
              <Button variant="secondary" size="sm" onClick={() => setImporting('plex')}>
                Import…
              </Button>
            </Row>
            <Row label="Jellyfin" description="Watched titles and resume points, for every user (sign in as an administrator).">
              <Button variant="secondary" size="sm" onClick={() => setImporting('jellyfin')}>
                Import…
              </Button>
            </Row>
          </Section>
          {importing && <ImportDialog source={importing} open onOpenChange={(o) => !o && setImporting(null)} />}

          <Section title="Skip Intro" hint="How Arc finds intros, recaps and credits. Each library chooses whether to offer Skip Intro and Skip Credits (Libraries › Edit).">
            <Row
              label="Chapters and IntroDB"
              description="Files with chapters named Opening or Ending are used as they are; otherwise episodes are looked up on IntroDB, a free community database. No key needed."
            />
            <KeyRow
              label="TheIntroDB"
              description="A bigger community database, looked up first when you add a free key."
              current={s.theintrodb_api_key}
              fixed={s.from_env.theintrodb_api_key}
              link="https://theintrodb.org"
              onSave={(v) => update.mutateAsync({ theintrodb_api_key: v })}
            />
            <ToggleRow
              label="Find intros and credits on this computer"
              description="When a title isn't in the databases: an episode is compared with the rest of its season to find the intro and credits they share, and a movie's closing credits are spotted from the picture. Runs in the maintenance time."
              checked={s.detect_intros}
              busy={update.isPending}
              onChange={(v) => update.mutate({ detect_intros: v })}
            />
          </Section>

          <Section title="Pre-roll" hint="A short clip before movies, like a cinema intro or a trailer, as Plex does.">
            <PrerollRow settings={s} onSave={(patch) => update.mutateAsync(patch)} />
          </Section>

          <Section title="Notifications" hint="Send messages to Discord, Slack or anywhere else when things happen.">
            <Webhooks />
          </Section>

          <Section title="Ratings" hint="Other sites' scores on title pages, and to sort by.">
            <Row
              label="IMDb"
              description="Downloaded from IMDb's free ratings file once a week, in the maintenance time. No key needed."
            />
            <KeyRow
              label="Rotten Tomatoes and Metacritic"
              description="Critics' scores, through OMDb (a free key allows 1,000 titles a day; Arc fetches a few each hour)."
              current={s.omdb_api_key}
              fixed={s.from_env.omdb_api_key}
              link="https://www.omdbapi.com/apikey.aspx"
              onSave={(v) => update.mutateAsync({ omdb_api_key: v })}
            />
          </Section>

          <Section title="Subtitles" hint="Find subtitles online from the player when a film or episode doesn't have the ones you want.">
            <KeyRow
              label="OpenSubtitles"
              description="Search and download subtitles."
              current={s.opensubtitles_api_key}
              fixed={s.from_env.opensubtitles_api_key}
              link="https://www.opensubtitles.com/en/consumers"
              onSave={(v) => update.mutateAsync({ opensubtitles_api_key: v })}
            />
            {s.opensubtitles_api_key && (
              <AccountRow username={s.opensubtitles_username} onSave={(username, password) => update.mutateAsync({ opensubtitles_username: username, opensubtitles_password: password })} />
            )}
          </Section>

          <Section title="Converting" hint="For players that can't play a file as it is, Arc converts it while it's watched.">
            <ToggleRow
              label="Use hardware acceleration"
              description={
                !s.transcoder.checked
                  ? 'Checking this computer…'
                  : s.transcoder.hw_encoder
                    ? `Convert with the graphics card (${s.transcoder.hw_label}): faster, and leaves the processor free. Arc falls back to the processor if the card can't handle a file.`
                    : 'No supported graphics card was found (NVIDIA, Intel, AMD or Apple), so Arc converts with the processor.'
              }
              checked={s.hw_accel && !!s.transcoder.hw_encoder}
              busy={update.isPending || !s.transcoder.hw_encoder}
              onChange={(v) => update.mutate({ hw_accel: v })}
            />
            <ToggleRow
              label="HDR tone mapping"
              description={
                s.transcoder.checked && !s.transcoder.tone_mapping
                  ? "This FFmpeg can't map HDR to SDR (it needs the zscale filter), so converted HDR video looks washed out."
                  : 'When converting HDR video, map its colours to SDR so it looks right on ordinary screens and in browsers.'
              }
              checked={s.tone_mapping && s.transcoder.tone_mapping}
              busy={update.isPending || !s.transcoder.tone_mapping}
              onChange={(v) => update.mutate({ tone_mapping: v })}
            />
          </Section>

          <Section
            title="Scheduled tasks"
            hint="Heavy background work waits for a quiet time of night, so it doesn't slow anyone watching."
          >
            <Row
              label="Maintenance time"
              description="Every night in this window Arc refreshes details that are a week (shows) or a month (movies) old, takes the daily backup, and checks every library for changes."
            >
              <div className="flex items-center gap-2 text-sm">
                <HourSelect value={s.maintenance_start} disabled={update.isPending} onChange={(h) => update.mutate({ maintenance_start: h })} />
                <span className="text-muted">to</span>
                <HourSelect value={s.maintenance_end} disabled={update.isPending} onChange={(h) => update.mutate({ maintenance_end: h })} />
              </div>
            </Row>
            <ToggleRow
              label="Make scrubbing previews only at this time"
              description="Otherwise they're made as soon as new files arrive, which keeps the processor busy for a while after adding a lot. Files someone starts watching always get theirs straight away."
              checked={s.previews_at_night}
              busy={update.isPending}
              onChange={(v) => update.mutate({ previews_at_night: v })}
            />
          </Section>

          {update.error && <p className="text-sm text-warning">{update.error.message}</p>}
        </>
      )}
    </>
  );
}

// Countries with their own age ratings, most likely first after sorting by name.
const regionCodes = ['AU', 'BR', 'CA', 'DE', 'DK', 'ES', 'FI', 'FR', 'GB', 'IE', 'IN', 'IT', 'JP', 'KR', 'MX', 'NL', 'NO', 'NZ', 'PL', 'PT', 'SE', 'US', 'ZA'];
const regionNames = new Intl.DisplayNames(['en'], { type: 'region' });

/** [code, name] pairs sorted by name, including the current region even if it isn't listed. */
export function regionOptions(current?: string): [string, string][] {
  const codes = current && !regionCodes.includes(current) ? [...regionCodes, current] : regionCodes;
  return codes.map((c) => [c, regionNames.of(c) ?? c] as [string, string]).sort((a, b) => a[1].localeCompare(b[1]));
}

/** The home's internet upload speed, shared between remote streams. */
function UploadRow({ kbps, onSave }: { kbps: number; onSave: (kbps: number) => Promise<unknown> }) {
  const [mbps, setMbps] = useState(kbps ? String(kbps / 1000) : '');
  const value = mbps.trim() === '' ? 0 : Math.round(Number(mbps) * 1000);
  const valid = mbps.trim() === '' || (Number.isFinite(value) && value >= 0);
  return (
    <Row
      label="Internet upload speed"
      description="Your connection's upload speed (a speed test shows it). Remote streams share 80% of it, so several at once still play."
    >
      <div className="flex items-center gap-2">
        <input value={mbps} onChange={(e) => setMbps(e.target.value)} inputMode="decimal" placeholder="Not set" className={`${inputClass} w-24 text-right`} />
        <span className="text-sm text-muted">Mbps</span>
        {valid && value !== kbps && (
          <Button variant="secondary" size="sm" onClick={() => onSave(value)}>
            Save
          </Button>
        )}
      </div>
    </Row>
  );
}

/** Remote access: on or off, the outside address and a check, the router, and HTTPS. */
function RemoteRows({ settings: s, onSave, busy }: { settings: ServerSettings; onSave: (p: Patch) => Promise<unknown>; busy: boolean }) {
  const status = useQuery({
    queryKey: ['remote', s.remote_access, s.upnp, s.remote_port, s.https_mode, s.https_domain, s.https_cert],
    queryFn: api.remote,
    refetchInterval: (q) => (q.state.data?.upnp.state === 'working' || q.state.data?.https.fetching ? 2000 : 30000),
  });
  const check = useMutation({ mutationFn: api.checkRemote });
  const [error, setError] = useState('');
  const save = async (p: Patch) => {
    setError('');
    try {
      await onSave(p);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };
  const r = status.data;
  return (
    <>
      <Row
        label="Remote access"
        description={
          <>
            Let people sign in from outside your home, like Plex&apos;s remote access. Your household signs in with the password, friends with their own.
            {error && <span className="mt-1 block text-warning">{error}</span>}
          </>
        }
      >
        <Switch checked={s.remote_access} disabled={busy} onCheckedChange={(v) => save({ remote_access: v })} aria-label="Remote access" />
      </Row>
      {s.remote_access && (
        <>
          <Row
            label="Outside address"
            description={
              !r ? (
                'Finding it…'
              ) : r.url ? (
                <>
                  <span className="font-mono text-foreground/80">{r.url}</span>
                  {r.double_nat && (
                    <span className="mt-1 flex items-start gap-1.5 text-warning">
                      <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
                      Your router is behind another router or your provider shares addresses, so people outside may not reach Arc even with the port forwarded.
                    </span>
                  )}
                  {check.data && (
                    <span className={`mt-1 flex items-start gap-1.5 ${check.data.reachable ? 'text-foreground' : 'text-warning'}`}>
                      {check.data.reachable ? <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-accent" /> : <XCircle className="mt-0.5 size-3.5 shrink-0" />}
                      {check.data.reachable
                        ? 'Arc answered at its outside address.'
                        : `${check.data.error}. Many routers can't reach their own outside address from inside the home, so try it on a phone with Wi-Fi off.`}
                    </span>
                  )}
                </>
              ) : (
                "Your home's outside address couldn't be found."
              )
            }
          >
            {r?.url && (
              <div className="flex gap-2">
                <Button size="sm" variant="ghost" onClick={() => navigator.clipboard?.writeText(r.url!)} aria-label="Copy address">
                  <Copy />
                </Button>
                <Button size="sm" variant="secondary" disabled={check.isPending} onClick={() => check.mutate()}>
                  {check.isPending && <Loader2 className="animate-spin" />} Check
                </Button>
              </div>
            )}
          </Row>
          <Row
            label="Open the port on the router"
            description={
              !s.upnp ? (
                <>
                  Forward port {s.remote_port} on your router to port {s.port} on this computer, in the router&apos;s settings.
                </>
              ) : r?.upnp.state === 'mapped' ? (
                <span className="inline-flex items-center gap-1 text-foreground/80">
                  <CheckCircle2 className="size-3.5 text-accent" /> Port {r.port} is forwarded{r.upnp.router ? ` by ${r.upnp.router}` : ''}
                </span>
              ) : r?.upnp.state === 'failed' ? (
                <span className="text-warning">{r.upnp.error}</span>
              ) : (
                'Asking the router (UPnP)…'
              )
            }
          >
            <Switch checked={s.upnp} disabled={busy} onCheckedChange={(v) => save({ upnp: v })} aria-label="Open the port with UPnP" />
          </Row>
          <OutsidePortRow settings={s} onSave={save} />
          <HTTPSRows settings={s} status={r?.https} onSave={save} />
        </>
      )}
    </>
  );
}

function OutsidePortRow({ settings: s, onSave }: { settings: ServerSettings; onSave: (p: Patch) => Promise<unknown> }) {
  const [port, setPort] = useState(String(s.remote_port));
  const n = Number(port);
  return (
    <Row
      label="Outside port"
      description={`The port people away from home connect to. Usually the same as Arc's port (${s.port}); Let's Encrypt needs 443.`}
    >
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          onSave({ remote_port: n });
        }}
      >
        <input value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, '').slice(0, 5))} inputMode="numeric" className={`${inputClass} w-24`} />
        {n !== s.remote_port && n > 0 && n < 65536 && (
          <Button size="sm" variant="primary" type="submit">
            Change
          </Button>
        )}
      </form>
    </Row>
  );
}

/** HTTPS: off, your own certificate, or Let's Encrypt; and how the certificate stands. */
function HTTPSRows({ settings: s, status, onSave }: { settings: ServerSettings; status?: RemoteStatus['https']; onSave: (p: Patch) => Promise<unknown> }) {
  const [mode, setMode] = useState(s.https_mode);
  const [cert, setCert] = useState(s.https_cert);
  const [key, setKey] = useState(s.https_key);
  const [domain, setDomain] = useState(s.https_domain);
  const [busy, setBusy] = useState(false);
  const changed = mode !== s.https_mode || (mode === 'files' && (cert !== s.https_cert || key !== s.https_key)) || (mode === 'letsencrypt' && domain !== s.https_domain);
  const apply = async () => {
    setBusy(true);
    try {
      await onSave(mode === 'files' ? { https_mode: mode, https_cert: cert, https_key: key } : mode === 'letsencrypt' ? { https_mode: mode, https_domain: domain } : { https_mode: '' });
    } finally {
      setBusy(false);
    }
  };
  const expires = status?.expires ? new Date(status.expires * 1000) : null;
  return (
    <div className="space-y-3 px-4 py-3.5">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
        <div className="min-w-0 flex-1 basis-64">
          <p className="text-sm font-medium">HTTPS</p>
          <div className="mt-0.5 text-xs leading-relaxed text-muted">
            {s.https_mode === '' ? (
              <span className="flex items-start gap-1.5 text-warning">
                <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
                Off: away from home, passwords and what you watch travel unencrypted. Turn it on before sharing your address.
              </span>
            ) : status?.error ? (
              <span className="text-warning">{status.error}</span>
            ) : status?.fetching ? (
              "Asking Let's Encrypt for a certificate…"
            ) : expires ? (
              <span className="inline-flex items-center gap-1 text-foreground/80">
                <CheckCircle2 className="size-3.5 text-accent" /> Certificate for {status?.names?.join(', ')}, until {expires.toLocaleDateString()}
                {s.https_mode === 'letsencrypt' && ' (renewed automatically)'}
              </span>
            ) : s.https_mode === 'letsencrypt' ? (
              'The certificate is fetched on the first visit to the domain.'
            ) : null}
            <span className="mt-1 block">On the same port as plain HTTP. At home both work; people away from home are sent to HTTPS.</span>
          </div>
        </div>
        <select value={mode} onChange={(e) => setMode(e.target.value as typeof mode)} className={`${inputClass} w-56 [&>option]:bg-surface`}>
          <option value="">Off</option>
          <option value="letsencrypt">Let&apos;s Encrypt (free)</option>
          <option value="files">My own certificate</option>
        </select>
      </div>
      {mode === 'letsencrypt' && (
        <div>
          <input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="arc.example.com" spellCheck={false} className={`${inputClass} w-full`} />
          <p className="mt-1.5 text-xs text-subtle">
            A domain name pointing at your home&apos;s address (a free dynamic DNS name works). Let&apos;s Encrypt checks it on port 443, so set the outside port to 443.
          </p>
        </div>
      )}
      {mode === 'files' && (
        <div className="grid gap-2">
          <input value={cert} onChange={(e) => setCert(e.target.value)} placeholder="Certificate file (.pem or .crt), full path" spellCheck={false} className={`${inputClass} w-full font-mono text-xs`} />
          <input value={key} onChange={(e) => setKey(e.target.value)} placeholder="Private key file (.pem or .key), full path" spellCheck={false} className={`${inputClass} w-full font-mono text-xs`} />
          <p className="text-xs text-subtle">PEM files, read again whenever they change, so another tool can renew them.</p>
        </div>
      )}
      {changed && (
        <div className="flex justify-end">
          <Button size="sm" variant="primary" disabled={busy} onClick={apply}>
            {busy && <Loader2 className="animate-spin" />} Save
          </Button>
        </div>
      )}
    </div>
  );
}

/** Pre-roll videos: files or folders, and whether one plays at random or all of them. */
function PrerollRow({ settings: s, onSave }: { settings: ServerSettings; onSave: (p: Patch) => Promise<unknown> }) {
  const [paths, setPaths] = useState(s.preroll_paths);
  const [busy, setBusy] = useState(false);
  const changed = paths.trim() !== s.preroll_paths;
  const save = async (patch: Patch) => {
    setBusy(true);
    try {
      await onSave(patch);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="space-y-3 px-4 py-3.5">
      <div>
        <p className="text-sm font-medium">Videos</p>
        <p className="mt-0.5 text-xs leading-relaxed text-muted">
          Files or folders of them, one per line. MP4 (H.264) plays everywhere. They play when a movie starts from the beginning, not when you
          carry on.
        </p>
      </div>
      <textarea
        value={paths}
        onChange={(e) => setPaths(e.target.value)}
        rows={3}
        placeholder={'D:\\Pre-rolls\nD:\\Trailers\\Arc intro.mp4'}
        className={`${inputClass} h-auto w-full resize-y py-2 font-mono text-xs leading-relaxed`}
      />
      <div className="flex flex-wrap items-center gap-3">
        <select
          value={s.preroll_mode}
          disabled={busy}
          onChange={(e) => save({ preroll_mode: e.target.value as 'random' | 'all' })}
          className={`${inputClass} [&>option]:bg-surface`}
        >
          <option value="random">Play one at random</option>
          <option value="all">Play them all, in order</option>
        </select>
        <span className="flex-1 text-xs text-muted">
          {s.preroll_paths ? (s.preroll_found ? `${s.preroll_found} video${s.preroll_found === 1 ? '' : 's'} found` : 'No videos found there') : 'Off'}
        </span>
        {changed && (
          <Button variant="primary" size="sm" disabled={busy} onClick={() => save({ preroll_paths: paths.trim() })}>
            {busy && <Loader2 className="animate-spin" />} Save
          </Button>
        )}
      </div>
    </div>
  );
}

/** An hour of the day, as 24-hour time. */
function HourSelect({ value, disabled, onChange }: { value: number; disabled: boolean; onChange: (h: number) => void }) {
  return (
    <select value={value} disabled={disabled} onChange={(e) => onChange(Number(e.target.value))} className={inputClass} aria-label="Hour">
      {Array.from({ length: 24 }, (_, h) => (
        <option key={h} value={h}>
          {String(h).padStart(2, '0')}:00
        </option>
      ))}
    </select>
  );
}

function Section({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <section>
      <h2 className="text-lg font-semibold">{title}</h2>
      {hint && <p className="mt-0.5 text-sm text-muted">{hint}</p>}
      <div className="mt-4 divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">{children}</div>
    </section>
  );
}

function Row({ label, description, children }: { label: string; description?: ReactNode; children?: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center gap-x-6 gap-y-3 px-4 py-3.5">
      <div className="min-w-0 flex-1 basis-64">
        <p className="text-sm font-medium">{label}</p>
        {description && <div className="mt-0.5 text-xs leading-relaxed text-muted">{description}</div>}
      </div>
      {children}
    </div>
  );
}

const inputClass = 'h-9 rounded-lg bg-surface-2/80 px-3 text-sm ring-1 ring-border outline-none focus:ring-2 focus:ring-accent';

function ToggleRow({ label, description, checked, busy, onChange }: { label: string; description: string; checked: boolean; busy: boolean; onChange: (v: boolean) => void }) {
  return (
    <Row label={label} description={description}>
      <Switch checked={checked} disabled={busy} onCheckedChange={onChange} aria-label={label} />
    </Row>
  );
}

/** Shows whether a key is set; "Change" reveals a field. Keys are checked with the service before saving. */
function KeyRow({ label, description, current, fixed, link, onSave }: { label: string; description: string; current: string; fixed?: boolean; link: string; onSave: (v: string) => Promise<unknown> }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const save = async (v: string) => {
    setBusy(true);
    setError('');
    try {
      await onSave(v);
      setEditing(false);
      setValue('');
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Row
      label={label}
      description={
        <>
          {description}{' '}
          {fixed ? (
            'Set by an environment variable.'
          ) : current ? (
            <span className="inline-flex items-center gap-1 text-foreground/80">
              <CheckCircle2 className="size-3.5 text-accent" /> Key {current}
            </span>
          ) : (
            <a href={link} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-accent hover:underline">
              Get a free key <ExternalLink className="size-3" />
            </a>
          )}
          {error && <span className="mt-1 block text-warning">{error}</span>}
        </>
      }
    >
      {!fixed &&
        (editing ? (
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              save(value.trim());
            }}
          >
            <input autoFocus value={value} onChange={(e) => setValue(e.target.value)} placeholder="Paste the key" className={`${inputClass} w-56`} spellCheck={false} />
            <Button size="sm" variant="primary" type="submit" disabled={!value.trim() || busy}>
              {busy && <Loader2 className="animate-spin" />} Save
            </Button>
            <Button size="sm" variant="ghost" type="button" onClick={() => setEditing(false)}>
              Cancel
            </Button>
          </form>
        ) : (
          <div className="flex gap-2">
            {current && (
              <Button size="sm" variant="ghost" onClick={() => save('')} disabled={busy}>
                Remove
              </Button>
            )}
            <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
              {current ? 'Change' : 'Add Key'}
            </Button>
          </div>
        ))}
    </Row>
  );
}

/** An optional OpenSubtitles account: more downloads a day than the key alone. */
function AccountRow({ username, onSave }: { username: string; onSave: (username: string, password: string) => Promise<unknown> }) {
  const [editing, setEditing] = useState(false);
  const [user, setUser] = useState('');
  const [pass, setPass] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const save = async (u: string, p: string) => {
    setBusy(true);
    setError('');
    try {
      await onSave(u, p);
      setEditing(false);
      setPass('');
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Row
      label="OpenSubtitles account"
      description={
        <>
          {username ? (
            <span className="inline-flex items-center gap-1 text-foreground/80">
              <CheckCircle2 className="size-3.5 text-accent" /> Signed in as {username}
            </span>
          ) : (
            'Optional: signing in allows more downloads a day.'
          )}
          {error && <span className="mt-1 block text-warning">{error}</span>}
        </>
      }
    >
      {editing ? (
        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            save(user.trim(), pass);
          }}
        >
          <input autoFocus value={user} onChange={(e) => setUser(e.target.value)} placeholder="Username" autoComplete="off" className={`${inputClass} w-36`} />
          <input value={pass} onChange={(e) => setPass(e.target.value)} type="password" placeholder="Password" autoComplete="new-password" className={`${inputClass} w-36`} />
          <Button size="sm" variant="primary" type="submit" disabled={!user.trim() || !pass || busy}>
            {busy && <Loader2 className="animate-spin" />} Sign In
          </Button>
          <Button size="sm" variant="ghost" type="button" onClick={() => setEditing(false)}>
            Cancel
          </Button>
        </form>
      ) : (
        <div className="flex gap-2">
          {username && (
            <Button size="sm" variant="ghost" onClick={() => save('', '')} disabled={busy}>
              Sign Out
            </Button>
          )}
          <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
            {username ? 'Change' : 'Sign In'}
          </Button>
        </div>
      )}
    </Row>
  );
}

/** Networks that count as home besides the usual private ones (Plex's "LAN networks"). */
function HomeNetworksRow({ value, onSave }: { value: string; onSave: (v: string) => Promise<unknown> }) {
  const [text, setText] = useState(value);
  const [error, setError] = useState('');
  const save = async (v: string) => {
    setError('');
    try {
      await onSave(v);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };
  const tailscale = '100.64.0.0/10';
  return (
    <Row
      label="Home networks"
      description={
        <>
          Other networks to treat as home, as well as this one: for skipping the password and for remote quality limits. For example a VPN, or
          Tailscale ({tailscale}).
          {!value.includes(tailscale) && (
            <button type="button" className="ml-1 text-accent hover:underline" onClick={() => (setText(value ? `${value}, ${tailscale}` : tailscale), void save(value ? `${value}, ${tailscale}` : tailscale))}>
              Add Tailscale
            </button>
          )}
          {error && <span className="mt-1 block text-warning">{error}</span>}
        </>
      }
    >
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void save(text);
        }}
      >
        <input value={text} onChange={(e) => setText(e.target.value)} placeholder="None" spellCheck={false} className={`${inputClass} w-56 font-mono text-xs`} />
        {text.trim() !== value && (
          <Button size="sm" variant="primary" type="submit">
            Save
          </Button>
        )}
      </form>
    </Row>
  );
}

function PortRow({ settings: s, onSave }: { settings: ServerSettings; onSave: (port: number) => Promise<unknown> }) {
  const [port, setPort] = useState(String(s.port));
  const [error, setError] = useState('');
  const [moved, setMoved] = useState('');
  const changed = Number(port) !== s.port;
  const host = s.allow_network ? window.location.hostname : 'localhost';
  return (
    <Row
      label="Port"
      description={
        <>
          Arc is at <span className="font-mono text-foreground/80">http://{host}:{s.port}</span>
          {moved && (
            <span className="mt-1 block text-foreground">
              Arc has moved.{' '}
              <a className="text-accent hover:underline" href={moved}>
                Open it at its new address
              </a>
              .
            </span>
          )}
          {error && <span className="mt-1 block text-warning">{error}</span>}
        </>
      }
    >
      <form
        className="flex gap-2"
        onSubmit={async (e) => {
          e.preventDefault();
          setError('');
          try {
            await onSave(Number(port));
            const url = new URL(window.location.href);
            url.port = port;
            setMoved(url.toString());
          } catch (err) {
            setError(err instanceof Error ? err.message : String(err));
          }
        }}
      >
        <input value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, '').slice(0, 5))} inputMode="numeric" className={`${inputClass} w-24`} />
        {changed && (
          <Button size="sm" variant="primary" type="submit">
            Change
          </Button>
        )}
      </form>
    </Row>
  );
}

function FFprobeRow({ settings: s, onSave }: { settings: ServerSettings; onSave: (path: string) => Promise<unknown> }) {
  const [path, setPath] = useState(s.ffprobe_path);
  const [error, setError] = useState('');
  const ok = s.ffprobe.found;
  return (
    <Row
      label="ffprobe"
      description={
        <>
          <span className="flex items-center gap-1.5">
            {ok ? <CheckCircle2 className="size-3.5 text-accent" /> : <XCircle className="size-3.5 text-warning" />}
            {ok ? s.ffprobe.version?.split(' Copyright')[0] : s.ffprobe.error}
          </span>
          {ok && <span className="mt-0.5 block truncate font-mono text-[11px] text-subtle">{s.ffprobe.path}</span>}
          {ok && (
            <span className="mt-1.5 flex items-center gap-1.5">
              {s.ffprobe.ffmpeg ? <CheckCircle2 className="size-3.5 shrink-0 text-accent" /> : <XCircle className="size-3.5 shrink-0 text-warning" />}
              {s.ffprobe.ffmpeg
                ? 'FFmpeg found: files a browser can’t play are converted as they’re watched'
                : 'FFmpeg wasn’t found next to ffprobe, so files a browser can’t play won’t be converted'}
            </span>
          )}
          {!ok && (
            <a href="https://ffmpeg.org/download.html" target="_blank" rel="noreferrer" className="mt-0.5 inline-flex items-center gap-1 text-accent hover:underline">
              Download FFmpeg <ExternalLink className="size-3" />
            </a>
          )}
          {error && <span className="mt-1 block text-warning">{error}</span>}
        </>
      }
    >
      <form
        className="flex w-full gap-2 sm:w-auto"
        onSubmit={async (e) => {
          e.preventDefault();
          setError('');
          try {
            await onSave(path.trim());
          } catch (err) {
            setError(err instanceof Error ? err.message : String(err));
          }
        }}
      >
        <input value={path} onChange={(e) => setPath(e.target.value)} placeholder="Found automatically" className={`${inputClass} min-w-0 flex-1 font-mono text-xs sm:w-72`} spellCheck={false} />
        {path !== s.ffprobe_path && (
          <Button size="sm" variant="primary" type="submit" disabled={s.from_env.ffprobe_path}>
            Save
          </Button>
        )}
      </form>
    </Row>
  );
}

function PasswordRow() {
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [done, setDone] = useState(false);
  const change = useMutation({
    mutationFn: () => api.changePassword(current, next),
    onSuccess: () => {
      setOpen(false);
      setCurrent('');
      setNext('');
      setDone(true);
    },
  });
  return (
    <Row
      label="Password"
      description={
        <>
          Used to sign in away from home, and for the admin profile.
          {done && <span className="mt-1 block text-foreground">Password changed. Other devices signed in with the old one were signed out.</span>}
          {change.error && <span className="mt-1 block text-warning">{change.error.message}</span>}
        </>
      }
    >
      {open ? (
        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            change.mutate();
          }}
        >
          <input type="password" value={current} onChange={(e) => setCurrent(e.target.value)} placeholder="Current" className={`${inputClass} w-36`} autoComplete="current-password" />
          <input type="password" value={next} onChange={(e) => setNext(e.target.value)} placeholder="New (8+ characters)" className={`${inputClass} w-44`} autoComplete="new-password" />
          <Button size="sm" variant="primary" type="submit" disabled={!current || next.length < 8 || change.isPending}>
            Change
          </Button>
        </form>
      ) : (
        <Button size="sm" variant="secondary" onClick={() => setOpen(true)}>
          Change Password
        </Button>
      )}
    </Row>
  );
}
