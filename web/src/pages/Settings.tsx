import { MaintenanceClock } from '@/components/MaintenanceClock';
import { LibraryNavigationSettings } from '@/components/LibraryNavigationSettings';
import { createContext, useContext, useState, type ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Copy, ExternalLink, Loader2, XCircle } from 'lucide-react';
import { api, type RemoteStatus, type ServerSettings, type SubtitleMode, type SubtitleStyle, type Theme } from '@/lib/api';
import { subtitleLook } from '@/components/Player';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { selectedSettingsSection, settingsSections as sectionGroups, type SettingsGroup } from '@/lib/settings-navigation';
import { useIsAdmin } from '@/lib/auth';
import { usePrefs, useUpdatePrefs } from '@/lib/home';
import { ErrorState, ErrorNotice, Page, PageHeader } from '@/components/Page';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { ImportDialog } from '@/components/ImportDialog';
import { qualities, qualityName } from '@/lib/playback';
import { Webhooks } from '@/components/Webhooks';
import { EmptyTrashControl } from '@/components/EmptyTrashTask';
import { MaintenanceTasks } from '@/components/ScheduledWork';
import { OptimizedList } from '@/components/Optimize';
import { DevicesList } from '@/components/Devices';
import { FolderPicker } from '@/components/FolderPicker';
import { Diagnostics } from '@/components/Diagnostics';

type Patch = Parameters<typeof api.updateSettings>[0];

const SettingsSection = createContext<SettingsGroup>('profile');

export function SettingsPage() {
  const admin = useIsAdmin();
  const [params] = useSearchParams();
  const selected = admin ? selectedSettingsSection(params.get('section')) : 'profile';
  return (
    <Page className="pb-16">
      <PageHeader title="Settings" subtitle={selected === 'profile' ? 'Your preferences follow your profile.' : 'Server settings apply to everyone. Only admins can change these.'} />
      <div className="max-w-[848px] space-y-8 px-6 md:px-10">
        <p className="text-sm text-muted">Switches and menus save immediately. Text fields use Save.</p>
        <SettingsSection.Provider value={selected}>
          {selected === 'profile' ? <ProfileSettings /> : admin && <ServerSettings key={selected} />}
        </SettingsSection.Provider>
      </div>
    </Page>
  );
}

/** The current profile's own settings (each profile has its own, as in Plex). */
function ProfileSettings() {
  const prefs = usePrefs();
  const update = useUpdatePrefs();
  const p = prefs.data;
  if (prefs.error && !p) return <ErrorState error={prefs.error} onRetry={() => void prefs.refetch()} />;
  if (!p) return <div className="h-48 animate-pulse rounded-2xl bg-surface" />;
  const busy = update.isPending;
  return (
    <SaveFeedbackContext.Provider value={{ keys: Object.keys(update.variables ?? {}), status: update.status, error: update.error, retry: () => { if (update.variables) update.mutate(update.variables); } }}>
      <Section title="Playback" hint="For your profile only.">
        <Row setting="watched_percent" label="Mark as watched at" description="Applies to future playback progress for your profile. It does not skip credits, stop playback or change earlier watch history.">
          <select aria-label="Mark as watched at" value={p.watched_percent || 90} disabled={busy} onChange={(e) => update.mutate({ watched_percent: Number(e.target.value) })} className={inputClass + ' w-56 [&>option]:bg-surface'}>
            {[75, 90, 95, 100].map(percent => <option key={percent} value={percent}>{percent}%{percent === 90 ? ' (default)' : ''}</option>)}
            {!!p.watched_percent && ![75, 90, 95, 100].includes(p.watched_percent) && <option value={p.watched_percent}>{p.watched_percent}%</option>}
          </select>
        </Row>
        <Row setting="audio_language" label="Preferred audio language" description="Used when a matching track is available. Choosing a language in the player also updates this preference.">
          <select aria-label="Preferred audio language" value={p.audio_language ?? ''} disabled={busy} onChange={(e) => update.mutate({ audio_language: e.target.value })} className={inputClass + ' w-56 [&>option]:bg-surface'}>
            <option value="">Use the file's default</option>
            {subtitleLanguages(p.audio_language ?? '').map(([code, name]) => <option key={code} value={code}>{name}</option>)}
          </select>
        </Row>
        <ToggleRow
          setting="auto_play" label="Auto-Play"
          description="Play the next episode after a short countdown."
          checked={p.auto_play !== false}
          busy={busy}
          onChange={(v) => update.mutate({ auto_play: v })}
        />
        <ToggleRow
          setting="auto_skip" label="Skip intros automatically"
          description="Use existing intro and recap markers to skip without pressing Skip. Marker detection is managed in Playback & transcoding."
          checked={!!p.auto_skip}
          busy={busy}
          onChange={(v) => update.mutate({ auto_skip: v })}
        />
        <ToggleRow
          setting="theme_music" label="Theme music"
          description="Play a show's or movie's theme quietly on its page, when there's one."
          checked={p.theme_music !== false}
          busy={busy}
          onChange={(v) => update.mutate({ theme_music: v })}
        />
      </Section>
      <LibraryNavigationSettings prefs={p} update={update} />
      <Section title="Subtitles" hint="Also in the player's Subtitles menu, where choosing a track sets your language.">
        <Row setting="subtitle_mode" label="Turn on by themselves" description={subtitleModeHints[p.subtitle_mode ?? '']}>
          <select
            aria-label="Subtitle mode"
            value={p.subtitle_mode ?? ''}
            disabled={busy}
            onChange={(e) => update.mutate({ subtitle_mode: e.target.value as SubtitleMode })}
            className={`${inputClass} w-56 [&>option]:bg-surface`}
          >
            <option value="">In the language I last chose</option>
            <option value="foreign">When the audio is in another language</option>
            <option value="always">Always</option>
            <option value="manual">Only when I choose them</option>
          </select>
        </Row>
        <Row setting="subtitle_language" label="Language" description="The subtitles to turn on. Forced subtitles (for foreign lines) show whatever is chosen here.">
          <select
            aria-label="Subtitle language"
            value={p.subtitle_language}
            disabled={busy}
            onChange={(e) => update.mutate({ subtitle_language: e.target.value })}
            className={`${inputClass} w-56 [&>option]:bg-surface`}
          >
            <option value="">None</option>
            {subtitleLanguages(p.subtitle_language).map(([code, name]) => (
              <option key={code} value={code}>
                {name}
              </option>
            ))}
          </select>
        </Row>
        <SubtitleStyleRows style={p.subtitle_style ?? {}} busy={busy} onChange={(subtitle_style) => update.mutate({ subtitle_style })} />
      </Section>
      <Section title="Display">
        <ToggleRow
          setting="hide_spoilers" label="Hide spoilers"
          description="Blur stills and hide descriptions of episodes you haven't seen."
          checked={!!p.hide_spoilers}
          busy={busy}
          onChange={(v) => update.mutate({ hide_spoilers: v })}
        />
        <Row setting="theme" label="Appearance" description="Automatic follows this device's light or dark setting.">
          <select
            aria-label="Appearance"
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
    </SaveFeedbackContext.Provider>
  );
}

const subtitleModeHints: Record<SubtitleMode, string> = {
  '': 'Subtitles come on in the language you last picked in the player, until you turn them off.',
  foreign: "Subtitles come on in your language when a film or episode's audio is in a different one (Plex's \"Shown with foreign audio\").",
  always: 'Subtitles come on in your language whenever there are some.',
  manual: 'Subtitles stay off unless you pick them in the player (forced subtitles still show).',
};

/** Common subtitle languages (ISO 639-2), with the profile's current one if it isn't among them. */
function subtitleLanguages(current: string): [string, string][] {
  const codes = ['eng', 'spa', 'fre', 'ger', 'ita', 'por', 'dut', 'swe', 'nor', 'dan', 'fin', 'pol', 'cze', 'hun', 'gre', 'tur', 'rus', 'ukr', 'ara', 'heb', 'hin', 'jpn', 'kor', 'chi', 'tha', 'vie', 'ind'];
  if (current && !codes.includes(current)) codes.unshift(current);
  return codes.map((c) => [c, fmt.language(c)] as [string, string]).sort((a, b) => a[1].localeCompare(b[1]));
}

/** How subtitles look, with a sample. */
function SubtitleStyleRows({ style, busy, onChange }: { style: SubtitleStyle; busy: boolean; onChange: (s: SubtitleStyle) => void }) {
  const pick = <K extends keyof SubtitleStyle>(key: K, options: [NonNullable<SubtitleStyle[K]>, string][], def: NonNullable<SubtitleStyle[K]>) => (
    <select
      value={style[key] ?? def}
      disabled={busy}
      onChange={(e) => onChange({ ...style, [key]: e.target.value })}
      className={`${inputClass} w-32 [&>option]:bg-surface`}
      aria-label={`Subtitle ${key}`}
    >
      {options.map(([v, label]) => (
        <option key={v} value={v}>
          {label}
        </option>
      ))}
    </select>
  );
  return (
    <div className="space-y-3 px-4 py-3.5">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
        <div className="min-w-0 flex-1 basis-64">
          <p className="text-sm font-medium">Style</p>
          <p className="mt-0.5 text-xs leading-relaxed text-muted">Size, colour, background and where they sit. For subtitles drawn as text; picture subtitles (from Blu-rays and DVDs) look as they were made.</p>
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        {pick('size', [['small', 'Small'], ['medium', 'Medium'], ['large', 'Large'], ['huge', 'Huge']], 'medium')}
        {pick('color', [['white', 'White'], ['yellow', 'Yellow']], 'white')}
        {pick('background', [['box', 'Shaded box'], ['solid', 'Black box'], ['none', 'Outline only']], 'box')}
        {pick('position', [['bottom', 'Bottom'], ['top', 'Top']], 'bottom')}
      </div>
      <SaveFeedback settings={['subtitle_style']} />
      <div className="flex h-24 items-end justify-center overflow-hidden rounded-xl bg-gradient-to-br from-slate-500 via-sky-800 to-amber-700 p-3" aria-hidden>
        <span className={cn('rounded-md px-2 py-0.5 text-lg font-medium', subtitleLook(style), style.position === 'top' && 'self-start')}>
          Where are we going?
        </span>
      </div>
    </div>
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
    <SaveFeedbackContext.Provider value={{ keys: Object.keys(update.variables ?? {}), status: update.status, error: update.error, retry: () => { if (update.variables) update.mutate(update.variables); } }}>
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
            <Row setting="region" label="Region" description="Age ratings (like 12A or PG-13) are shown for this country and used by parental controls.">
              <select
                aria-label="Region"
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

          <Section title="Library scanning" hint="Keep all your libraries up to date. Applies to every library.">
            <ToggleRow
              setting="scan_automatic" label="Scan automatically when files change"
              description="Find new or changed media when your folders report a change."
              checked={s.scan_automatic}
              busy={update.isPending}
              onChange={(v) => update.mutate({ scan_automatic: v })}
            />
            <Row
              setting="scan_interval_minutes" label="Full library scan"
              description="Check all library folders for changes, including changes your drives may not report. Intervals run from server start or when this setting changes."
            >
              <select
                aria-label="Full library scan"
                value={s.scan_interval_minutes}
                disabled={update.isPending}
                onChange={(e) => update.mutate({ scan_interval_minutes: Number(e.target.value) })}
                className={inputClass + ' w-44 [&>option]:bg-surface'}
              >
                <option value={0}>Off</option>
                <option value={15}>Every 15 minutes</option>
                <option value={30}>Every 30 minutes</option>
                <option value={60}>Every hour</option>
                <option value={120}>Every 2 hours</option>
                <option value={360}>Every 6 hours</option>
                <option value={720}>Every 12 hours</option>
                <option value={1440}>Every 24 hours</option>
              </select>
            </Row>
            <p className="px-5 py-3 text-sm text-muted">Some network drives do not report changes. Keep a scheduled scan enabled to catch them.</p>
            {s.scan_interval_minutes === 0 && (
              <p className="px-5 pb-3 text-sm text-muted" role="status">
                {!s.scan_automatic && !s.scan_on_start
                  ? 'Libraries update when you scan them or change their folders.'
                  : s.scan_automatic && s.scan_on_start
                    ? 'Full scans are off. Arc still scans detected changes and checks libraries at startup.'
                    : s.scan_automatic
                      ? 'Full scans are off. Arc still scans detected changes.'
                      : 'Full scans are off. Arc still checks libraries at startup.'} Running scans finish normally.
              </p>
            )}
            <details className="px-5 py-3">
              <summary className="cursor-pointer text-sm font-medium">Advanced scanning options</summary>
              <ToggleRow
                setting="scan_partial" label="Scan only changed folders"
                description="For detected changes, check the affected folders instead of the whole library."
                checked={s.scan_partial}
                busy={update.isPending || !s.scan_automatic}
                onChange={(v) => update.mutate({ scan_partial: v })}
              />
              <ToggleRow
                setting="scan_on_start" label="Scan when Arc starts"
                description="Check all libraries for changes when the server starts."
                checked={s.scan_on_start}
                busy={update.isPending}
                onChange={(v) => update.mutate({ scan_on_start: v })}
              />
            </details>
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

          <Section title="Notifications" hint="Send library and playback events to Discord, Slack or another webhook destination.">
            <Webhooks />
          </Section>

          <Section title="Media tools" hint="Arc uses ffprobe (part of FFmpeg) to read what's inside your video files, and FFmpeg to convert them for players that need it.">
            <FFprobeRow settings={s} onSave={(path) => update.mutateAsync({ ffprobe_path: path })} />
          </Section>

          <Section title="Intro & credit detection" hint="Where the markers for Skip Intro, Recap and Credits come from. Each library chooses whether to offer them (Manage libraries › Edit), and automatic skipping is a preference in Your profile.">
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
            <Row label="Find intros and credits on this computer" description={<>When there are no chapters or online markers, Arc compares a season&apos;s episodes or looks for movie credits itself. It is a scheduled task: choose when it runs in Maintenance &amp; diagnostics. Currently: {processingLabels[s.detection_mode]}.</>}>
              <Link to="/settings?section=maintenance#settings-scheduled-tasks" className="text-sm font-medium text-accent hover:underline">Scheduled tasks</Link>
            </Row>
          </Section>

          <Section title="Pre-roll" hint="A short clip before movies, like a cinema intro or a trailer, as Plex does.">
            <PrerollRow settings={s} onSave={(patch) => update.mutateAsync(patch)} />
          </Section>
          {importing && <ImportDialog source={importing} open onOpenChange={(o) => !o && setImporting(null)} />}

          <Section title="Subtitle downloads" hint="Server download services; subtitle language and appearance are in Your profile. Find subtitles online from the player when a film or episode doesn't have the ones you want.">
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
              setting="hw_accel" label="Use hardware acceleration"
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
              setting="tone_mapping" label="HDR tone mapping"
              description={
                s.transcoder.checked && !s.transcoder.tone_mapping
                  ? "This FFmpeg can't map HDR to SDR (it needs the zscale filter), so converted HDR video looks washed out."
                  : 'When converting HDR video, map its colours to SDR so it looks right on ordinary screens and in browsers.'
              }
              checked={s.tone_mapping && s.transcoder.tone_mapping}
              busy={update.isPending || !s.transcoder.tone_mapping}
              onChange={(v) => update.mutate({ tone_mapping: v })}
            />
            <Row setting="max_transcodes" label="Videos converted at once" description="More at once needs a faster computer. Anyone over the limit is told to try again shortly; files that play as they are, or only need repackaging, don't count.">
              <select
                aria-label="Videos converted at once"
            value={s.max_transcodes}
                disabled={update.isPending}
                onChange={(e) => update.mutate({ max_transcodes: Number(e.target.value) })}
                className={`${inputClass} w-40 [&>option]:bg-surface`}
              >
                <option value={0}>No limit</option>
                {[1, 2, 3, 4, 6, 8, 10].map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </Row>
            <TranscodeDirRow value={s.transcode_dir} onSave={(v) => update.mutateAsync({ transcode_dir: v })} />
          </Section>

          <Section title="Optimized Versions" hint="Easy-to-play copies made ahead of time, played instead of converting live when a player needs them.">
            <OptimizedList />
          </Section>

          <Section title="Sign-in" hint="Away from home, the password is always needed.">
            <ToggleRow
              setting="local_bypass" label="Skip the password at home"
              description="On this computer and your home network, people just pick a profile. The admin profile still asks for the password or its PIN."
              checked={s.local_bypass}
              busy={update.isPending}
              onChange={(v) => update.mutate({ local_bypass: v })}
            />
            <PasswordRow />
          </Section>

          <Section title="Network">
            <ToggleRow
              setting="allow_network" label="Allow other devices"
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
            <Row setting="remote_kbps" label="Limit each remote stream" description="Videos are converted down to this when they're bigger, so they play smoothly on slower connections.">
              <select
                aria-label="Limit each remote stream"
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

          <Section
            title="Scheduled tasks"
            hint="Heavy background work waits for a quiet time of night, so it doesn't slow anyone watching."
          >
            <Row
              setting={['maintenance_start', 'maintenance_end']} label="Maintenance time"
              description="Enabled tasks run in these hours, using the server’s local time. Work resumes next time if it cannot finish. New-file discovery follows Library scanning settings."
            >
              <div className="flex items-center gap-2 text-sm">
                <HourSelect label="Maintenance start" value={s.maintenance_start} disabled={update.isPending} onChange={(h) => update.mutate({ maintenance_start: h })} />
                <span className="text-muted">to</span>
                <HourSelect label="Maintenance end" value={s.maintenance_end} disabled={update.isPending} onChange={(h) => update.mutate({ maintenance_end: h })} />
              </div>
            </Row>
            <MaintenanceClock start={s.maintenance_start} end={s.maintenance_end} />
            <MaintenanceTasks
              rows={[
                {
                  kind: 'previews', label: 'Scrubbing previews',
                  description: 'Timeline pictures for libraries with previews turned on (Manage libraries › Edit › Advanced).',
                  control: <ProcessingSelect label="Scrubbing preview processing" value={s.preview_mode} busy={update.isPending} onChange={v => update.mutate({ preview_mode: v })} />,
                  feedback: <SaveFeedback settings={['preview_mode']} />,
                },
                {
                  kind: 'markers', label: 'Intro & credit detection',
                  description: 'Compare episodes or analyse movie credits on this computer when chapters and online databases have no markers.',
                  control: <ProcessingSelect label="Local intro and credits detection" value={s.detection_mode} busy={update.isPending} onChange={v => update.mutate({ detection_mode: v })} />,
                  feedback: <SaveFeedback settings={['detection_mode']} />,
                },
                {
                  kind: 'analysis', label: 'Media analysis',
                  description: 'Fill in missing HDR and audio format details for existing files. New files are analysed when added.',
                  control: <Switch checked={s.analysis_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ analysis_enabled: v })} aria-label="Update existing media analysis" />,
                  feedback: <SaveFeedback settings={['analysis_enabled']} />,
                },
                {
                  kind: 'metadata', label: 'Metadata refresh',
                  description: 'Refresh ageing title details and airing shows, without scanning media folders.',
                  control: <Switch checked={s.metadata_refresh_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ metadata_refresh_enabled: v })} aria-label="Refresh known-title metadata" />,
                  feedback: <SaveFeedback settings={['metadata_refresh_enabled']} />,
                },
                {
                  kind: 'backup', label: 'Database backups',
                  description: 'A daily database backup. Manual and safety backups are always available in Backups & restore.',
                  control: <Switch checked={s.auto_backups_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ auto_backups_enabled: v })} aria-label="Automatic database backups" />,
                  feedback: <SaveFeedback settings={['auto_backups_enabled']} />,
                },
                {
                  kind: 'database', label: 'Optimize database',
                  description: 'Weekly during maintenance: check database integrity, refresh query statistics and compact substantial unused space when enough disk space is available. Playback and scanning take priority.',
                  control: <Switch checked={s.database_upkeep_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ database_upkeep_enabled: v })} aria-label="Optimize database automatically" />,
                  feedback: <SaveFeedback settings={['database_upkeep_enabled']} />,
                },
                {
                  kind: 'cache', label: 'Clean up cached files',
                  description: 'Weekly during maintenance: remove unused cached images, previews and subtitles. Recent cache files and titles on offline drives are kept. Media files, uploaded artwork and backups are untouched.',
                  control: <Switch checked={s.cache_cleanup_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ cache_cleanup_enabled: v })} aria-label="Clean up cached files automatically" />,
                  feedback: <SaveFeedback settings={['cache_cleanup_enabled']} />,
                },
                {
                  kind: 'empty_trash', label: 'Empty trash automatically',
                  description: 'Off by default. During maintenance after a fresh complete scan, permanently remove missing records and their watch history. Every folder needs a readable known file; otherwise cleanup waits. A safety backup is required. Media files are never deleted.',
                  control: <EmptyTrashControl enabled={s.empty_trash_enabled} busy={update.isPending} onChange={v => update.mutateAsync({ empty_trash_enabled: v })} />,
                  feedback: <SaveFeedback settings={['empty_trash_enabled']} />,
                  action: <Link className="ml-auto inline-flex h-8 shrink-0 items-center rounded-lg bg-surface-2 px-3 text-[13px] font-medium hover:bg-surface-3 sm:ml-0" to="/libraries">Choose library…</Link>,
                },
              ]}
            />
          </Section>

          <Section title="Diagnostics" hint="Capture useful information when something is slow or fails."><Diagnostics /></Section>

        </>
      )}
    </SaveFeedbackContext.Provider>
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
  const pending = useContext(SaveFeedbackContext)?.status === 'pending';
  const save = async () => {
    try { await onSave(value); } catch { /* Row feedback provides the error and retry. */ }
  };
  const [mbps, setMbps] = useState(kbps ? String(kbps / 1000) : '');
  const value = mbps.trim() === '' ? 0 : Math.round(Number(mbps) * 1000);
  const valid = mbps.trim() === '' || (Number.isFinite(value) && value >= 0);
  return (
    <Row
      setting="upload_kbps" label="Internet upload speed"
      description="Your connection's upload speed (a speed test shows it). Remote streams share 80% of it, so several at once still play."
    >
      <div className="flex items-center gap-2">
        <input aria-label="Internet upload speed" disabled={pending} value={mbps} onChange={(e) => setMbps(e.target.value)} inputMode="decimal" placeholder="Not set" className={`${inputClass} w-24 text-right`} />
        <span className="text-sm text-muted">Mbps</span>
        {valid && value !== kbps && (
          <Button variant="secondary" size="sm" disabled={pending} onClick={() => void save()}>
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
  const save = async (p: Patch) => {
    try {
      await onSave(p);
    } catch {
      // Each remote-setting row displays the shared mutation error and retry.
    }
  };
  const r = status.data;
  return (
    <>
      <Row
        compact setting="remote_access" label="Remote access"
        description={
          <>
            Let people sign in from outside your home, like Plex&apos;s remote access. Your household signs in with the password, friends with their own.
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
            compact setting="upnp" label="Open the port on the router"
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
      setting="remote_port" label="Outside port"
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
        <select aria-label="HTTPS mode" value={mode} onChange={(e) => setMode(e.target.value as typeof mode)} className={`${inputClass} w-56 [&>option]:bg-surface`}>
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
      <SaveFeedback settings={['https_mode', 'https_domain', 'https_cert', 'https_key']} />
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
    } catch {
      // The mutation feedback below exposes the failure and retry.
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
          aria-label="Pre-roll order"
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
      <SaveFeedback settings={['preroll_paths', 'preroll_mode']} />
    </div>
  );
}

/** An hour of the day, as 24-hour time. */
function HourSelect({ value, disabled, onChange, label }: { label: string; value: number; disabled: boolean; onChange: (h: number) => void }) {
  return (
    <select value={value} disabled={disabled} onChange={(e) => onChange(Number(e.target.value))} className={inputClass} aria-label={label}>
      {Array.from({ length: 24 }, (_, h) => (
        <option key={h} value={h}>
          {String(h).padStart(2, '0')}:00
        </option>
      ))}
    </select>
  );
}

function sectionId(title: string) { return 'settings-' + title.toLowerCase().replace(/[^a-z0-9]+/g, '-'); }

function Section({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  const group = useContext(SettingsSection);
  if (group !== 'profile' && sectionGroups[title] !== group) return null;
  return (
    <section id={sectionId(title)} className="scroll-mt-24 lg:scroll-mt-6">
      <h2 className="text-lg font-semibold">{title}</h2>
      {hint && <p className="mt-0.5 text-sm text-muted">{hint}</p>}
      <div className="mt-4 divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">{children}</div>
    </section>
  );
}

type SaveState = { keys: string[]; status: 'idle' | 'pending' | 'error' | 'success'; error: unknown; retry: () => void };
const SaveFeedbackContext = createContext<SaveState | null>(null);

function SaveFeedback({ settings, className }: { settings: string[]; className?: string }) {
  const save = useContext(SaveFeedbackContext);
  if (!save || !settings.some(key => save.keys.includes(key))) return null;
  if (save.status === 'error') return <div className={className}><ErrorNotice error={save.error} onRetry={save.retry} /></div>;
  if (save.status === 'idle') return null;
  return <p role="status" className={cn('text-xs text-muted', className)}>{save.status === 'pending' ? 'Saving…' : 'Saved'}</p>;
}

function Row({ label, description, children, setting, compact = false }: { label: string; description?: ReactNode; children?: ReactNode; setting?: string | string[]; compact?: boolean }) {
  return (
    <div className={cn('flex items-center gap-x-6 gap-y-3 px-4 py-3.5', !compact && 'flex-wrap')}>
      <div className={cn('min-w-0 flex-1', !compact && 'basis-64')}>
        <p className="text-sm font-medium">{label}</p>
        {description && <div className="mt-0.5 text-xs leading-relaxed text-muted">{description}</div>}
        {compact && setting && <SaveFeedback className="mt-2" settings={typeof setting === 'string' ? [setting] : setting} />}
      </div>
      {children}
      {!compact && setting && <SaveFeedback className="w-full" settings={typeof setting === 'string' ? [setting] : setting} />}
    </div>
  );
}

const inputClass = 'h-9 max-w-full rounded-lg bg-surface-2/80 px-3 text-sm ring-1 ring-border outline-none focus:ring-2 focus:ring-accent';

function ToggleRow({ label, description, checked, busy, onChange, setting }: { label: string; description: string; checked: boolean; busy: boolean; onChange: (v: boolean) => void; setting?: string }) {
  return (
    <Row compact setting={setting} label={label} description={description}>
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
          {error && <span role="alert" className="mt-1 block text-warning">{error}</span>}
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
          {error && <span role="alert" className="mt-1 block text-warning">{error}</span>}
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

/** Where converted streams are written while they're watched. */
function TranscodeDirRow({ value, onSave }: { value: string; onSave: (v: string) => Promise<unknown> }) {
  const [picking, setPicking] = useState(false);
  const [error, setError] = useState('');
  const save = async (v: string) => {
    setError('');
    try {
      await onSave(v);
      setPicking(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };
  return (
    <>
      <Row
        label="Temporary folder"
        description={
          <>
            Where converted video is kept while it&apos;s watched (deleted afterwards). A fast drive with free space is best.{' '}
            <span className="font-mono text-foreground/80">{value || "Arc's data folder"}</span>
            {error && <span role="alert" className="mt-1 block text-warning">{error}</span>}
          </>
        }
      >
        <div className="flex gap-2">
          {value && (
            <Button size="sm" variant="ghost" onClick={() => void save('')}>
              Use Default
            </Button>
          )}
          <Button size="sm" variant="secondary" onClick={() => setPicking(true)}>
            Choose…
          </Button>
        </div>
      </Row>
      {picking && (
        <div className="px-4 pb-4">
          <FolderPicker start={value || undefined} onChoose={(p) => void save(p)} onCancel={() => setPicking(false)} />
        </div>
      )}
    </>
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
          {error && <span role="alert" className="mt-1 block text-warning">{error}</span>}
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
          {error && <span role="alert" className="mt-1 block text-warning">{error}</span>}
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
          {error && <span role="alert" className="mt-1 block text-warning">{error}</span>}
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
          {change.error && <span role="alert" className="mt-1 block text-warning">{change.error.message}</span>}
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

const processingLabels: Record<ServerSettings['preview_mode'], string> = { off: 'Off', scheduled: 'During maintenance', added: 'When added and during maintenance' };

function ProcessingSelect({label,value,busy,onChange}:{label:string;value:ServerSettings['preview_mode'];busy:boolean;onChange:(value:ServerSettings['preview_mode'])=>void}) {
 return <select aria-label={label} value={value} disabled={busy} onChange={e=>onChange(e.target.value as ServerSettings['preview_mode'])} className={inputClass+' min-w-0 flex-1 sm:w-72 sm:flex-none [&>option]:bg-surface'}>
  {(Object.keys(processingLabels) as ServerSettings['preview_mode'][]).map(v => <option key={v} value={v}>{processingLabels[v]}</option>)}
 </select>;
}
