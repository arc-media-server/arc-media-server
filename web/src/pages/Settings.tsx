import { useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ExternalLink, Loader2, XCircle } from 'lucide-react';
import { api, type ServerSettings } from '@/lib/api';
import { ErrorState, Page, PageHeader } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';

type Patch = Parameters<typeof api.updateSettings>[0];

export function SettingsPage() {
  const qc = useQueryClient();
  const settings = useQuery({ queryKey: ['settings'], queryFn: api.settings });
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
    <Page className="pb-16">
      <PageHeader title="Settings" subtitle="Everything about how this Arc server runs." />
      {!s ? (
        <div className="mx-6 h-96 animate-pulse rounded-2xl bg-surface md:mx-10" />
      ) : (
        <div className="max-w-3xl space-y-10 px-6 md:px-10">
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
          </Section>

          <Section title="Media tools" hint="Arc uses ffprobe (part of FFmpeg) to read what's inside your video files, and FFmpeg to convert them for players that need it.">
            <FFprobeRow settings={s} onSave={(path) => update.mutateAsync({ ffprobe_path: path })} />
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

          {update.error && <p className="text-sm text-warning">{update.error.message}</p>}
        </div>
      )}
    </Page>
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
