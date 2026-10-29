import i18n from '@/i18n';
import { Trans, useTranslation } from 'react-i18next';
import { MaintenanceClock } from '@/components/MaintenanceClock';
import { LibraryNavigationSettings } from '@/components/LibraryNavigationSettings';
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Copy, ExternalLink, Loader2, XCircle } from 'lucide-react';
import { api, type RemoteStatus, type ServerSettings, type SubtitleMode, type SubtitleStyle, type Theme } from '@/lib/api';
import { subtitleLook } from '@/components/Player';
import * as fmt from '@/lib/format';
import { cn } from '@/lib/utils';
import { mediaToolsNeeded, selectedSettingsSection, settingsSections as sectionGroups, type SettingsGroup } from '@/lib/settings-navigation';
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
import { ProviderCredits } from '@/components/ProviderCredits';

type Patch = Parameters<typeof api.updateSettings>[0];

const SettingsSection = createContext<SettingsGroup>('profile');

export function SettingsPage() {
  const { t } = useTranslation('profileSettings');
  const admin = useIsAdmin();
  const [params] = useSearchParams();
  const selected = admin ? selectedSettingsSection(params.get('section')) : 'profile';
  return (
    <Page className="pb-16">
      <PageHeader title={t("profileSettings:settings.settings")} subtitle={selected === 'profile' ? t('profileSettings:settings.profileDescription') : t('profileSettings:settings.serverDescription')} />
      <div className="max-w-[848px] space-y-8 px-6 md:px-10">
        <p className="text-sm text-muted">{t("profileSettings:settings.switchesAndMenusSave")}</p>
        <SettingsSection.Provider value={selected}>
          {selected === 'profile' ? <ProfileSettings /> : admin && <ServerSettings key={selected} />}
        </SettingsSection.Provider>
        {selected === 'profile' && <AboutArc />}
      </div>
    </Page>
  );
}

/** About Arc, on every profile's Settings: the version, and the credits metadata providers require. */
function AboutArc() {
  const { t } = useTranslation('profileSettings');
  const info = useQuery({ queryKey: ['server-info'], queryFn: api.serverInfo, staleTime: Infinity });
  return (
    <section aria-labelledby="about-arc">
      <h2 id="about-arc" className="text-lg font-semibold">{t("profileSettings:settings.aboutArc")}</h2>
      <p className="mt-1 text-sm text-muted">{t("profileSettings:settings.arcMediaServer")}{info.data?.version ? ` ${info.data.version}` : ''}</p>
      <div className="mt-4 rounded-2xl bg-surface p-4 ring-1 ring-border">
        <h3 className="mb-3 text-sm font-medium">{t("profileSettings:settings.credits")}</h3>
        <ProviderCredits />
      </div>
    </section>
  );
}

/** The current profile's own settings (each profile has its own, as in Plex). */
function ProfileSettings() {
  const { t } = useTranslation('profileSettings');
  const subtitleModeHints: Record<SubtitleMode, string> = { '': t('profileSettings:subtitleHints.last'), foreign: t('profileSettings:subtitleHints.foreign'), always: t('profileSettings:subtitleHints.always'), manual: t('profileSettings:subtitleHints.manual') };
  const prefs = usePrefs();
  const update = useUpdatePrefs();
  const p = prefs.data;
  if (prefs.error && !p) return <ErrorState error={prefs.error} onRetry={() => void prefs.refetch()} />;
  if (!p) return <div className="h-48 animate-pulse rounded-2xl bg-surface" />;
  const busy = update.isPending;
  return (
    <SaveFeedbackContext.Provider value={{ keys: Object.keys(update.variables ?? {}), status: update.status, error: update.error, retry: () => { if (update.variables) update.mutate(update.variables); } }}>
      <Section sectionKey="Playback" title={t("profileSettings:settings.playback")} hint={t("profileSettings:settings.forYourProfileOnly")}>
        <Row setting="watched_percent" label={t("profileSettings:settings.markAsWatchedAt")} description={t("profileSettings:settings.howFarThroughA")}>
          <select aria-label={t("profileSettings:settings.markAsWatchedAt")} value={p.watched_percent || 90} disabled={busy} onChange={(e) => update.mutate({ watched_percent: Number(e.target.value) })} className={inputClass + ' w-56 [&>option]:bg-surface'}>
            {[75, 90, 95, 100].map(percent => <option key={percent} value={percent}>{percent === 90 ? t('profileSettings:settings.defaultPercent', { percent }) : `${percent}%`}</option>)}
            {!!p.watched_percent && ![75, 90, 95, 100].includes(p.watched_percent) && <option value={p.watched_percent}>{p.watched_percent}%</option>}
          </select>
        </Row>
        <Row setting="audio_language" label={t("profileSettings:settings.preferredAudioLanguage")} description={t("profileSettings:settings.playedWhenAVideo")}>
          <select aria-label={t("profileSettings:settings.preferredAudioLanguage")} value={p.audio_language ?? ''} disabled={busy} onChange={(e) => update.mutate({ audio_language: e.target.value })} className={inputClass + ' w-56 [&>option]:bg-surface'}>
            <option value="">{t("profileSettings:settings.useTheFileS")}</option>
            {subtitleLanguages(p.audio_language ?? '').map(([code, name]) => <option key={code} value={code}>{name}</option>)}
          </select>
        </Row>
        <ToggleRow
          setting="auto_play" label={t("profileSettings:settings.autoPlay")}
          description={t("profileSettings:settings.playTheNextEpisode")}
          checked={p.auto_play !== false}
          busy={busy}
          onChange={(v) => update.mutate({ auto_play: v })}
        />
        <ToggleRow
          setting="auto_skip" label={t("profileSettings:settings.skipIntrosAutomatically")}
          description={t("profileSettings:settings.skipIntrosAndRecaps")}
          checked={!!p.auto_skip}
          busy={busy}
          onChange={(v) => update.mutate({ auto_skip: v })}
        />
        <ToggleRow
          setting="theme_music" label={t("profileSettings:settings.themeMusic")}
          description={t("profileSettings:settings.playAShowS")}
          checked={p.theme_music !== false}
          busy={busy}
          onChange={(v) => update.mutate({ theme_music: v })}
        />
      </Section>
      <Section sectionKey="Home" title={t("profileSettings:settings.home")} hint={t("profileSettings:settings.forYourProfileOnly")}>
        <ToggleRow setting="show_home_hero" label={t("profileSettings:settings.showHomeHero")} description={t("profileSettings:settings.showTheLargeFeatured")}
          checked={p.show_home_hero !== false} busy={busy} onChange={v => update.mutate({ show_home_hero: v })} />
      </Section>
      <LibraryNavigationSettings prefs={p} update={update} />
      <Section sectionKey="Subtitles" title={t("profileSettings:settings.subtitles")} hint={t("profileSettings:settings.choosingSubtitlesInThe")}>
        <Row setting="subtitle_mode" label={t("profileSettings:settings.showSubtitles")} description={subtitleModeHints[p.subtitle_mode ?? '']}>
          <select
            aria-label={t("profileSettings:settings.subtitleMode")}
            value={p.subtitle_mode ?? ''}
            disabled={busy}
            onChange={(e) => update.mutate({ subtitle_mode: e.target.value as SubtitleMode })}
            className={`${inputClass} w-56 [&>option]:bg-surface`}
          >
            <option value="">{t("profileSettings:settings.inTheLanguageI")}</option>
            <option value="foreign">{t("profileSettings:settings.whenTheAudioIs")}</option>
            <option value="always">{t("profileSettings:settings.always")}</option>
            <option value="manual">{t("profileSettings:settings.onlyWhenIChoose")}</option>
          </select>
        </Row>
        <Row setting="subtitle_language" label={t("profileSettings:settings.language")} description={t("profileSettings:settings.theLanguageOfSubtitles")}>
          <select
            aria-label={t("profileSettings:settings.subtitleLanguage")}
            value={p.subtitle_language}
            disabled={busy}
            onChange={(e) => update.mutate({ subtitle_language: e.target.value })}
            className={`${inputClass} w-56 [&>option]:bg-surface`}
          >
            <option value="">{t("profileSettings:settings.none")}</option>
            {subtitleLanguages(p.subtitle_language).map(([code, name]) => (
              <option key={code} value={code}>
                {name}
              </option>
            ))}
          </select>
        </Row>
        <SubtitleStyleRows style={p.subtitle_style ?? {}} busy={busy} onChange={(subtitle_style) => update.mutate({ subtitle_style })} />
      </Section>
      <Section sectionKey="Display" title={t("profileSettings:settings.display")}>
        <ToggleRow
          setting="hide_spoilers" label={t("profileSettings:settings.hideSpoilers")}
          description={t("profileSettings:settings.blurStillsAndHide")}
          checked={!!p.hide_spoilers}
          busy={busy}
          onChange={(v) => update.mutate({ hide_spoilers: v })}
        />
        <Row setting="theme" label={t("profileSettings:settings.appearance")} description={t("profileSettings:settings.automaticFollowsThisDevice")}>
          <select
            aria-label={t("profileSettings:settings.appearance")}
            value={p.theme ?? ''}
            disabled={busy}
            onChange={(e) => update.mutate({ theme: e.target.value as Theme })}
            className={`${inputClass} w-40 [&>option]:bg-surface`}
          >
            <option value="">{t("profileSettings:settings.automatic")}</option>
            <option value="dark">{t("profileSettings:settings.dark")}</option>
            <option value="light">{t("profileSettings:settings.light")}</option>
          </select>
        </Row>
      </Section>
    </SaveFeedbackContext.Provider>
  );
}



/** Common subtitle languages (ISO 639-2), with the profile's current one if it isn't among them. */
function subtitleLanguages(current: string): [string, string][] {
  const codes = ['eng', 'spa', 'fre', 'ger', 'ita', 'por', 'dut', 'swe', 'nor', 'dan', 'fin', 'pol', 'cze', 'hun', 'gre', 'tur', 'rus', 'ukr', 'ara', 'heb', 'hin', 'jpn', 'kor', 'chi', 'tha', 'vie', 'ind'];
  if (current && !codes.includes(current)) codes.unshift(current);
  return codes.map((c) => [c, fmt.language(c)] as [string, string]).sort((a, b) => a[1].localeCompare(b[1]));
}

/** How subtitles look, with a sample. */
function SubtitleStyleRows({ style, busy, onChange }: { style: SubtitleStyle; busy: boolean; onChange: (s: SubtitleStyle) => void }) {
  const { t } = useTranslation('profileSettings');
  const pick = <K extends keyof SubtitleStyle>(key: K, options: [NonNullable<SubtitleStyle[K]>, string][], def: NonNullable<SubtitleStyle[K]>) => (
    <select
      value={style[key] ?? def}
      disabled={busy}
      onChange={(e) => onChange({ ...style, [key]: e.target.value })}
      className={`${inputClass} w-32 [&>option]:bg-surface`}
      aria-label={{ size: t('profileSettings:styleLabels.size'), color: t('profileSettings:styleLabels.color'), background: t('profileSettings:styleLabels.background'), position: t('profileSettings:styleLabels.position') }[key]}
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
          <p className="text-sm font-medium">{t("profileSettings:settings.style")}</p>
          <p className="mt-0.5 text-xs leading-relaxed text-muted">{t("profileSettings:settings.sizeColourBackgroundAnd")}</p>
        </div>
      </div>
      <div className="flex flex-wrap gap-2">
        {pick('size', [['small', t('profileSettings:styleOptions.small')], ['medium', t('profileSettings:styleOptions.medium')], ['large', t('profileSettings:styleOptions.large')], ['huge', t('profileSettings:styleOptions.huge')]], 'medium')}
        {pick('color', [['white', t('profileSettings:styleOptions.white')], ['yellow', t('profileSettings:styleOptions.yellow')]], 'white')}
        {pick('background', [['box', t('profileSettings:styleOptions.box')], ['solid', t('profileSettings:styleOptions.solid')], ['none', t('profileSettings:styleOptions.outline')]], 'box')}
        {pick('position', [['bottom', t('profileSettings:styleOptions.bottom')], ['top', t('profileSettings:styleOptions.top')]], 'bottom')}
      </div>
      <SaveFeedback settings={['subtitle_style']} />
      <div className="flex h-24 items-end justify-center overflow-hidden rounded-xl bg-gradient-to-br from-slate-500 via-sky-800 to-amber-700 p-3" aria-hidden>
        <span className={cn('rounded-md px-2 py-0.5 text-lg font-medium', subtitleLook(style), style.position === 'top' && 'self-start')}>{t("profileSettings:settings.whereAreWeGoing")}</span>
      </div>
    </div>
  );
}

/** The server's settings (admin). */
function ServerSettings() {
  const { t } = useTranslation('serverSettings');
  const detectionHelp = {
    off: t('serverSettings:detectionHelp.off'),
    scheduled: t('serverSettings:detectionHelp.scheduled'),
    added: t('serverSettings:detectionHelp.added'),
  };
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
      <h2 className="border-t border-border pt-8 text-xs font-semibold uppercase tracking-wider text-subtle">{t("serverSettings:settings.server")}</h2>
      {!s ? (
        <div className="h-96 animate-pulse rounded-2xl bg-surface" />
      ) : (
        <>
          <Section sectionKey="Online metadata" title={t("serverSettings:settings.onlineMetadata")} hint={t("serverSettings:settings.whereArcGetsDetails")}>
            <KeyRow
              label={t("serverSettings:settings.tmdb")}
              description={t("serverSettings:settings.moviesAndShows")}
              current={s.tmdb_api_key}
              fixed={s.from_env.tmdb_api_key}
              link="https://www.themoviedb.org/settings/api"
              onSave={(v) => update.mutateAsync({ tmdb_api_key: v })}
            />
            <KeyRow
              label={t("serverSettings:settings.thetvdb")}
              description={t("serverSettings:settings.showsAndTheirEpisode")}
              current={s.tvdb_api_key}
              fixed={s.from_env.tvdb_api_key}
              link="https://thetvdb.com/api-information"
              onSave={(v) => update.mutateAsync({ tvdb_api_key: v })}
            />
          </Section>
          <Section sectionKey="Ratings" title={t("serverSettings:settings.ratings")} hint={t("serverSettings:settings.scoresFromOtherSites")}>
            <KeyRow
              label={t("serverSettings:settings.rottenTomatoesAndMetacritic")}
              description={t("serverSettings:settings.criticsScoresFromOmdb")}
              current={s.omdb_api_key}
              fixed={s.from_env.omdb_api_key}
              link="https://www.omdbapi.com/apikey.aspx"
              onSave={(v) => update.mutateAsync({ omdb_api_key: v })}
            />
          </Section>

          <Section sectionKey="Library scanning" title={t("serverSettings:settings.libraryScanning")} hint={t("serverSettings:settings.howArcFindsNew")}>
            <ToggleRow
              setting="scan_automatic" label={t("serverSettings:settings.scanAutomaticallyWhenFiles")}
              description={t("serverSettings:settings.pickUpNewAnd")}
              checked={s.scan_automatic}
              busy={update.isPending}
              onChange={(v) => update.mutate({ scan_automatic: v })}
            />
            <Row
              setting="scan_interval_minutes" label={t("serverSettings:settings.fullLibraryScan")}
              description={t("serverSettings:settings.regularlyCheckEveryFolder")}
            >
              <select
                aria-label={t("serverSettings:settings.fullLibraryScan")}
                value={s.scan_interval_minutes}
                disabled={update.isPending}
                onChange={(e) => update.mutate({ scan_interval_minutes: Number(e.target.value) })}
                className={inputClass + ' w-44 [&>option]:bg-surface'}
              >
                <option value={0}>{t("serverSettings:settings.off")}</option>
                <option value={15}>{t("serverSettings:settings.every15Minutes")}</option>
                <option value={30}>{t("serverSettings:settings.every30Minutes")}</option>
                <option value={60}>{t("serverSettings:settings.everyHour")}</option>
                <option value={120}>{t("serverSettings:settings.every2Hours")}</option>
                <option value={360}>{t("serverSettings:settings.every6Hours")}</option>
                <option value={720}>{t("serverSettings:settings.every12Hours")}</option>
                <option value={1440}>{t("serverSettings:settings.every24Hours")}</option>
              </select>
            </Row>
            {s.scan_interval_minutes === 0 && (
              <p className="px-5 pb-3 text-sm text-muted" role="status">
                {!s.scan_automatic && !s.scan_on_start
                  ? t("serverSettings:settings.librariesUpdateWhenYou")
                  : s.scan_automatic && s.scan_on_start
                    ? t("serverSettings:settings.fullScansAreOff")
                    : s.scan_automatic
                      ? t("serverSettings:settings.fullScansAreOff2")
                      : t("serverSettings:settings.fullScansAreOff3")}
              </p>
            )}
            <details className="px-5 py-3">
              <summary className="cursor-pointer text-sm font-medium">{t("serverSettings:settings.advancedScanningOptions")}</summary>
              <ToggleRow
                setting="scan_partial" label={t("serverSettings:settings.scanOnlyChangedFolders")}
                description={t("serverSettings:settings.whenAChangeIs")}
                checked={s.scan_partial}
                busy={update.isPending || !s.scan_automatic}
                onChange={(v) => update.mutate({ scan_partial: v })}
              />
              <ToggleRow
                setting="scan_on_start" label={t("serverSettings:settings.scanWhenArcStarts")}
                description={t("serverSettings:settings.checkEveryLibraryFor")}
                checked={s.scan_on_start}
                busy={update.isPending}
                onChange={(v) => update.mutate({ scan_on_start: v })}
              />
            </details>
          </Section>

          <Section sectionKey="Moving in" title={t("serverSettings:settings.movingIn")} hint={t("serverSettings:settings.bringEveryoneSWatch")}>
            <Row label={t("serverSettings:settings.plex")} description={t("serverSettings:settings.watchedStatusResumePoints")}>
              <Button variant="secondary" size="sm" onClick={() => setImporting('plex')}>{t("serverSettings:settings.import")}</Button>
            </Row>
            <Row label={t("serverSettings:settings.jellyfin")} description={t("serverSettings:settings.watchedStatusAndResume")}>
              <Button variant="secondary" size="sm" onClick={() => setImporting('jellyfin')}>{t("serverSettings:settings.import")}</Button>
            </Row>
          </Section>

          <Section sectionKey="Notifications" title={t("serverSettings:settings.notifications")} hint={t("serverSettings:settings.sendMessagesToDiscord")}>
            <Webhooks />
          </Section>

          {mediaToolsNeeded(s) && <Section sectionKey="Media tools" title={t("serverSettings:settings.mediaTools")} hint={t("serverSettings:settings.ffmpegReadsYourVideo")}>
            <FFprobeRow settings={s} onSave={(path) => update.mutateAsync({ ffprobe_path: path })} />
          </Section>}

          <Section sectionKey="Intro & credit detection" title={t("serverSettings:settings.introCreditDetection")} hint={t("serverSettings:settings.whereSkipIntroRecap")}>
            <Row
              label={t("serverSettings:settings.chaptersAndIntrodb")}
              description={t("serverSettings:settings.chaptersNamedOpeningOr")}
            />
            <KeyRow
              label={t("serverSettings:settings.theintrodb")}
              description={t("serverSettings:settings.aLargerCommunityDatabase")}
              current={s.theintrodb_api_key}
              fixed={s.from_env.theintrodb_api_key}
              link="https://theintrodb.org"
              onSave={(v) => update.mutateAsync({ theintrodb_api_key: v })}
            />
            <Row label={t("serverSettings:settings.detectIntrosAndCredits")} description={detectionHelp[s.detection_mode]}>
              <Link to="/settings?section=maintenance#settings-scheduled-tasks" className="text-sm font-medium text-accent hover:underline">{t("serverSettings:settings.scheduledTasks")}</Link>
            </Row>
          </Section>

          <Section sectionKey="Pre-roll" title={t("serverSettings:settings.preRoll")} hint={t("serverSettings:settings.aShortClipPlayed")}>
            <PrerollRow settings={s} onSave={(patch) => update.mutateAsync(patch)} />
          </Section>
          {importing && <ImportDialog source={importing} open onOpenChange={(o) => !o && setImporting(null)} />}

          <Section sectionKey="Subtitle downloads" title={t("serverSettings:settings.subtitleDownloads")} hint={t("serverSettings:settings.findSubtitlesOnlineFrom")}>
            <KeyRow
              label={t("serverSettings:settings.openSubtitles")}
              description={t("serverSettings:settings.searchAndDownloadSubtitles")}
              current={s.opensubtitles_api_key}
              fixed={s.from_env.opensubtitles_api_key}
              link="https://www.opensubtitles.com/en/consumers"
              onSave={(v) => update.mutateAsync({ opensubtitles_api_key: v })}
            />
            {s.opensubtitles_api_key && (
              <AccountRow username={s.opensubtitles_username} onSave={(username, password) => update.mutateAsync({ opensubtitles_username: username, opensubtitles_password: password })} />
            )}
          </Section>

          <Section sectionKey="Converting" title={t("serverSettings:settings.converting")} hint={t("serverSettings:settings.whenADeviceCan")}>
            <ToggleRow
              setting="hw_accel" label={t("serverSettings:settings.useHardwareAcceleration")}
              description={
                !s.transcoder.checked
                  ? t("serverSettings:settings.checkingThisComputer")
                  : s.transcoder.hw_encoder
                    ? t('serverSettings:settings.hardwareDescription', { hardware: s.transcoder.hw_label })
                    : t("serverSettings:settings.noSupportedGraphicsCard")
              }
              checked={s.hw_accel && !!s.transcoder.hw_encoder}
              busy={update.isPending || !s.transcoder.hw_encoder}
              onChange={(v) => update.mutate({ hw_accel: v })}
            />
            <ToggleRow
              setting="tone_mapping" label={t("serverSettings:settings.hdrToneMapping")}
              description={
                s.transcoder.checked && !s.transcoder.tone_mapping
                  ? t("serverSettings:settings.thisFfmpegCanT")
                  : t("serverSettings:settings.keepConvertedHdrVideo")
              }
              checked={s.tone_mapping && s.transcoder.tone_mapping}
              busy={update.isPending || !s.transcoder.tone_mapping}
              onChange={(v) => update.mutate({ tone_mapping: v })}
            />
            <Row setting="max_transcodes" label={t("serverSettings:settings.videosConvertedAtOnce")} description={t("serverSettings:settings.howManyVideosArc")}>
              <select
                aria-label={t("serverSettings:settings.videosConvertedAtOnce")}
            value={s.max_transcodes}
                disabled={update.isPending}
                onChange={(e) => update.mutate({ max_transcodes: Number(e.target.value) })}
                className={`${inputClass} w-40 [&>option]:bg-surface`}
              >
                <option value={0}>{t("serverSettings:settings.noLimit")}</option>
                {[1, 2, 3, 4, 6, 8, 10].map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </Row>
            <TranscodeDirRow value={s.transcode_dir} onSave={(v) => update.mutateAsync({ transcode_dir: v })} />
          </Section>

          <Section sectionKey="Optimized Versions" title={t("serverSettings:settings.optimizedVersions")} hint={t("serverSettings:settings.easyToPlayCopies")}>
            <OptimizedList />
          </Section>

          <Section sectionKey="Sign-in" title={t("serverSettings:settings.signIn")} hint={t("serverSettings:settings.awayFromHomeThe")}>
            <ToggleRow
              setting="local_bypass" label={t("serverSettings:settings.skipThePasswordAt")}
              description={t("serverSettings:settings.atHomePeopleJust")}
              checked={s.local_bypass}
              busy={update.isPending}
              onChange={(v) => update.mutate({ local_bypass: v })}
            />
            <PasswordRow />
          </Section>

          <Section sectionKey="Network" title={t("serverSettings:settings.network")}>
            <ToggleRow
              setting="allow_network" label={t("serverSettings:settings.allowOtherDevices")}
              description={t("serverSettings:settings.letPhonesTvsAnd")}
              checked={s.allow_network}
              busy={update.isPending}
              onChange={(v) => update.mutate({ allow_network: v })}
            />
            <PortRow settings={s} onSave={(port) => update.mutateAsync({ port })} />
            <HomeNetworksRow value={s.home_networks} onSave={(v) => update.mutateAsync({ home_networks: v })} />
            <TrustedProxiesRow value={s.trusted_proxies} onSave={(v) => update.mutateAsync({ trusted_proxies: v })} />
          </Section>

          <Section sectionKey="Remote Access" title={t("serverSettings:settings.remoteAccess")} hint={t("serverSettings:settings.watchAwayFromHome")}>
            <RemoteRows settings={s} onSave={(p) => update.mutateAsync(p)} busy={update.isPending} />
            <Row setting="remote_kbps" label={t("serverSettings:settings.limitEachRemoteStream")} description={t("serverSettings:settings.biggerVideosAreConverted")}>
              <select
                aria-label={t("serverSettings:settings.limitEachRemoteStream")}
            value={s.remote_kbps}
                disabled={update.isPending}
                onChange={(e) => update.mutate({ remote_kbps: Number(e.target.value) })}
                className={`${inputClass} w-48 [&>option]:bg-surface`}
              >
                <option value={0}>{t("serverSettings:settings.originalNoLimit")}</option>
                {qualities.map((q) => (
                  <option key={q} value={q}>
                    {qualityName(q)}
                  </option>
                ))}
              </select>
            </Row>
            <UploadRow kbps={s.upload_kbps} onSave={(kbps) => update.mutateAsync({ upload_kbps: kbps })} />
          </Section>

          <Section sectionKey="Devices" title={t("serverSettings:settings.devices")} hint={t("serverSettings:settings.browsersAndAppsSigned")}>
            <DevicesList />
          </Section>

          <Section sectionKey="Scheduled tasks" title={t("serverSettings:settings.scheduledTasks")}
            hint={t("serverSettings:settings.heavyWorkWaitsFor")}
          >
            <Row
              setting={['maintenance_start', 'maintenance_end']} label={t("serverSettings:settings.maintenanceTime")}
              description={<>
                <span>{s.maintenance_start === s.maintenance_end ? t("serverSettings:settings.tasksCanRunAt") : t("serverSettings:settings.tasksRunBetweenThese")}</span>
                <MaintenanceClock start={s.maintenance_start} end={s.maintenance_end} />
              </>}
            >
              <div className="flex items-center gap-2 text-sm">
                <HourSelect label={t("serverSettings:settings.maintenanceStart")} value={s.maintenance_start} disabled={update.isPending} onChange={(h) => update.mutate({ maintenance_start: h })} />
                <span className="text-muted">{t("serverSettings:settings.to")}</span>
                <HourSelect label={t("serverSettings:settings.maintenanceEnd")} value={s.maintenance_end} disabled={update.isPending} onChange={(h) => update.mutate({ maintenance_end: h })} />
              </div>
            </Row>
            <MaintenanceTasks
              rows={[
                {
                  kind: 'previews', label: t('serverSettings:tasks.previewsLabel'),
                  description: t('serverSettings:tasks.previewsDescription'),
                  control: <ProcessingSelect label={t("serverSettings:settings.scrubbingPreviews")} value={s.preview_mode} busy={update.isPending} onChange={v => update.mutate({ preview_mode: v })} />,
                  feedback: <SaveFeedback settings={['preview_mode']} />,
                },
                {
                  kind: 'markers', label: t('serverSettings:tasks.markersLabel'),
                  description: t('serverSettings:tasks.markersDescription'),
                  control: <ProcessingSelect label={t("serverSettings:settings.introCreditDetection")} value={s.detection_mode} busy={update.isPending} onChange={v => update.mutate({ detection_mode: v })} />,
                  feedback: <SaveFeedback settings={['detection_mode']} />,
                },
                {
                  kind: 'analysis', label: t('serverSettings:tasks.analysisLabel'),
                  description: t('serverSettings:tasks.analysisDescription'),
                  control: <Switch checked={s.analysis_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ analysis_enabled: v })} aria-label={t("serverSettings:settings.mediaAnalysis")} />,
                  feedback: <SaveFeedback settings={['analysis_enabled']} />,
                },
                {
                  kind: 'metadata', label: t('serverSettings:tasks.metadataLabel'),
                  description: t('serverSettings:tasks.metadataDescription'),
                  control: <Switch checked={s.metadata_refresh_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ metadata_refresh_enabled: v })} aria-label={t("serverSettings:settings.metadataRefresh")} />,
                  feedback: <SaveFeedback settings={['metadata_refresh_enabled']} />,
                },
                {
                  kind: 'backup', label: t('serverSettings:tasks.backupLabel'),
                  description: t('serverSettings:tasks.backupDescription'),
                  control: <Switch checked={s.auto_backups_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ auto_backups_enabled: v })} aria-label={t("serverSettings:settings.databaseBackups")} />,
                  feedback: <SaveFeedback settings={['auto_backups_enabled']} />,
                },
                {
                  kind: 'database', label: t('serverSettings:tasks.databaseLabel'),
                  description: t('serverSettings:tasks.databaseDescription'),
                  control: <Switch checked={s.database_upkeep_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ database_upkeep_enabled: v })} aria-label={t("serverSettings:settings.optimizeDatabaseAutomatically")} />,
                  feedback: <SaveFeedback settings={['database_upkeep_enabled']} />,
                },
                {
                  kind: 'cache', label: t('serverSettings:tasks.cacheLabel'),
                  description: t('serverSettings:tasks.cacheDescription'),
                  control: <Switch checked={s.cache_cleanup_enabled} disabled={update.isPending} onCheckedChange={v => update.mutate({ cache_cleanup_enabled: v })} aria-label={t("serverSettings:settings.cleanUpCachedFiles")} />,
                  feedback: <SaveFeedback settings={['cache_cleanup_enabled']} />,
                },
                {
                  kind: 'empty_trash', label: t('serverSettings:tasks.emptyTrashLabel'),
                  description: t('serverSettings:tasks.emptyTrashDescription'),
                  control: <EmptyTrashControl enabled={s.empty_trash_enabled} busy={update.isPending} onChange={v => update.mutateAsync({ empty_trash_enabled: v })} />,
                  feedback: <SaveFeedback settings={['empty_trash_enabled']} />,
                  action: <Link className="ml-auto inline-flex h-8 shrink-0 items-center rounded-lg bg-surface-2 px-3 text-[13px] font-medium hover:bg-surface-3 sm:ml-0" to="/libraries">{t("serverSettings:settings.chooseLibrary")}</Link>,
                },
              ]}
            />
          </Section>

          <Section sectionKey="Diagnostics" title={t("serverSettings:settings.diagnostics")} hint={t("serverSettings:settings.informationToHelpWhen")}><Diagnostics /></Section>

        </>
      )}
    </SaveFeedbackContext.Provider>
  );
}

/** The home's internet upload speed, shared between remote streams. */
function UploadRow({ kbps, onSave }: { kbps: number; onSave: (kbps: number) => Promise<unknown> }) {
  const { t } = useTranslation('accessSettings');
  const pending = useContext(SaveFeedbackContext)?.status === 'pending';
  const save = async () => {
    try { await onSave(value); } catch { /* Row feedback provides the error and retry. */ }
  };
  const [mbps, setMbps] = useState(kbps ? String(kbps / 1000) : '');
  const value = mbps.trim() === '' ? 0 : Math.round(Number(mbps) * 1000);
  const valid = mbps.trim() === '' || (Number.isFinite(value) && value >= 0);
  return (
    <Row
      setting="upload_kbps" label={t("accessSettings:settings.internetUploadSpeed")}
      description={t("accessSettings:settings.fromASpeedTest")}
    >
      <div className="flex items-center gap-2">
        <input aria-label={t("accessSettings:settings.internetUploadSpeed")} disabled={pending} value={mbps} onChange={(e) => setMbps(e.target.value)} inputMode="decimal" placeholder={t("accessSettings:settings.notSet")} className={`${inputClass} w-24 text-right`} />
        <span className="text-sm text-muted">{t("accessSettings:settings.mbps")}</span>
        {valid && value !== kbps && (
          <Button variant="secondary" size="sm" disabled={pending} onClick={() => void save()}>{t("accessSettings:settings.save")}</Button>
        )}
      </div>
    </Row>
  );
}

/** Remote access: on or off, the outside address and a check, the router, and HTTPS. */
function RemoteRows({ settings: s, onSave, busy }: { settings: ServerSettings; onSave: (p: Patch) => Promise<unknown>; busy: boolean }) {
  const { t } = useTranslation('accessSettings');
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
        compact setting="remote_access" label={t("accessSettings:settings.remoteAccess")}
        description={
          <>{t("accessSettings:settings.letYourHouseholdAnd")}</>
        }
      >
        <Switch checked={s.remote_access} disabled={busy} onCheckedChange={(v) => save({ remote_access: v })} aria-label={t("accessSettings:settings.remoteAccess")} />
      </Row>
      {s.remote_access && (
        <>
          <Row
            label={t("accessSettings:settings.outsideAddress")}
            description={
              !r ? (
                t("accessSettings:settings.findingIt")
              ) : r.url ? (
                <>
                  <span className="font-mono text-foreground/80">{r.url}</span>
                  {r.double_nat && (
                    <span className="mt-1 flex items-start gap-1.5 text-warning">
                      <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />{t("accessSettings:settings.yourRouterIsBehind")}</span>
                  )}
                  {check.data && (
                    <span className={`mt-1 flex items-start gap-1.5 ${check.data.reachable ? 'text-foreground' : 'text-warning'}`}>
                      {check.data.reachable ? <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-accent" /> : <XCircle className="mt-0.5 size-3.5 shrink-0" />}
                      {check.data.reachable
                        ? t("accessSettings:settings.arcAnsweredAtIts")
                        : t('accessSettings:settings.checkError', { error: check.data.error })}
                    </span>
                  )}
                </>
              ) : (
                t("accessSettings:settings.yourHomeSOutside")
              )
            }
          >
            {r?.url && (
              <div className="flex gap-2">
                <Button size="sm" variant="ghost" onClick={() => navigator.clipboard?.writeText(r.url!)} aria-label={t("accessSettings:settings.copyAddress")}>
                  <Copy />
                </Button>
                <Button size="sm" variant="secondary" disabled={check.isPending} onClick={() => check.mutate()}>
                  {check.isPending && <Loader2 className="animate-spin" />} {t("accessSettings:settings.check")}</Button>
              </div>
            )}
          </Row>
          <Row
            compact setting="upnp" label={t("accessSettings:settings.openThePortOn")}
            description={
              !s.upnp ? (
                <>{t('accessSettings:settings.forwardDescription', { outside: s.remote_port, inside: s.port })}</>
              ) : r?.upnp.state === 'mapped' ? (
                <span className="inline-flex items-center gap-1 text-foreground/80">
                  <CheckCircle2 className="size-3.5 text-accent" /> {r.upnp.router ? t('accessSettings:settings.forwardedRouter', { port: r.port, router: r.upnp.router }) : t('accessSettings:settings.forwarded', { port: r.port })}
                </span>
              ) : r?.upnp.state === 'failed' ? (
                <span className="text-warning">{r.upnp.error}</span>
              ) : (
                t("accessSettings:settings.askingTheRouterUpnp")
              )
            }
          >
            <Switch checked={s.upnp} disabled={busy} onCheckedChange={(v) => save({ upnp: v })} aria-label={t("accessSettings:settings.openThePortWith")} />
          </Row>
          <OutsidePortRow settings={s} onSave={save} />
          <HTTPSRows settings={s} status={r?.https} onSave={save} />
        </>
      )}
    </>
  );
}

function OutsidePortRow({ settings: s, onSave }: { settings: ServerSettings; onSave: (p: Patch) => Promise<unknown> }) {
  const { t } = useTranslation('accessSettings');
  const [port, setPort] = useState(String(s.remote_port));
  const n = Number(port);
  return (
    <Row
      setting="remote_port" label={t("accessSettings:settings.outsidePort")}
      description={t('accessSettings:settings.outsidePortDescription', { port: s.port })}
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
          <Button size="sm" variant="primary" type="submit">{t("accessSettings:settings.change")}</Button>
        )}
      </form>
    </Row>
  );
}

/** HTTPS: off, your own certificate, or Let's Encrypt; and how the certificate stands. */
function HTTPSRows({ settings: s, status, onSave }: { settings: ServerSettings; status?: RemoteStatus['https']; onSave: (p: Patch) => Promise<unknown> }) {
  const { t } = useTranslation('accessSettings');
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
          <p className="text-sm font-medium">{t("accessSettings:settings.https")}</p>
          <div className="mt-0.5 text-xs leading-relaxed text-muted">
            {s.https_mode === '' ? (
              <span className="flex items-start gap-1.5 text-warning">
                <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />{t("accessSettings:settings.offAwayFromHome")}</span>
            ) : status?.error ? (
              <span className="text-warning">{status.error}</span>
            ) : status?.fetching ? (
              t("accessSettings:settings.askingLetSEncrypt")
            ) : expires ? (
              <span className="inline-flex items-center gap-1 text-foreground/80">
                <CheckCircle2 className="size-3.5 text-accent" /> {s.https_mode === 'letsencrypt' ? t('accessSettings:settings.automaticCertificate', { names: status?.names?.join(', '), date: fmt.localDate(expires) }) : t('accessSettings:settings.certificate', { names: status?.names?.join(', '), date: fmt.localDate(expires) })}
              </span>
            ) : s.https_mode === 'letsencrypt' ? (
              t("accessSettings:settings.theCertificateIsFetched")
            ) : null}
            <span className="mt-1 block">{t("accessSettings:settings.onTheSamePort")}</span>
          </div>
        </div>
        <select aria-label={t("accessSettings:settings.httpsMode")} value={mode} onChange={(e) => setMode(e.target.value as typeof mode)} className={`${inputClass} w-56 [&>option]:bg-surface`}>
          <option value="">{t("accessSettings:settings.off")}</option>
          <option value="letsencrypt">{t("accessSettings:settings.letSEncryptFree")}</option>
          <option value="files">{t("accessSettings:settings.myOwnCertificate")}</option>
        </select>
      </div>
      {mode === 'letsencrypt' && (
        <div>
          <input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder={t("accessSettings:settings.arcExampleCom")} spellCheck={false} className={`${inputClass} w-full`} />
          <p className="mt-1.5 text-xs text-subtle">{t("accessSettings:settings.aDomainNamePointing")}</p>
        </div>
      )}
      {mode === 'files' && (
        <div className="grid gap-2">
          <input value={cert} onChange={(e) => setCert(e.target.value)} placeholder={t("accessSettings:settings.certificateFilePemOr")} spellCheck={false} className={`${inputClass} w-full font-mono text-xs`} />
          <input value={key} onChange={(e) => setKey(e.target.value)} placeholder={t("accessSettings:settings.privateKeyFilePem")} spellCheck={false} className={`${inputClass} w-full font-mono text-xs`} />
          <p className="text-xs text-subtle">{t("accessSettings:settings.pemFilesReadAgain")}</p>
        </div>
      )}
      {changed && (
        <div className="flex justify-end">
          <Button size="sm" variant="primary" disabled={busy} onClick={apply}>
            {busy && <Loader2 className="animate-spin" />} {t("accessSettings:settings.save")}</Button>
        </div>
      )}
      <SaveFeedback settings={['https_mode', 'https_domain', 'https_cert', 'https_key']} />
    </div>
  );
}

/** Pre-roll videos: files or folders, and whether one plays at random or all of them. */
function PrerollRow({ settings: s, onSave }: { settings: ServerSettings; onSave: (p: Patch) => Promise<unknown> }) {
  const { t } = useTranslation('serverSettings');
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
        <p className="text-sm font-medium">{t("serverSettings:settings.videos")}</p>
        <p className="mt-0.5 text-xs leading-relaxed text-muted">{t("serverSettings:settings.filesOrFoldersOne")}</p>
      </div>
      <textarea
        value={paths}
        onChange={(e) => setPaths(e.target.value)}
        rows={3}
        placeholder={t("serverSettings:settings.dPreRollsD")}
        className={`${inputClass} h-auto w-full resize-y py-2 font-mono text-xs leading-relaxed`}
      />
      <div className="flex flex-wrap items-center gap-3">
        <select
          aria-label={t("serverSettings:settings.preRollOrder")}
            value={s.preroll_mode}
          disabled={busy}
          onChange={(e) => save({ preroll_mode: e.target.value as 'random' | 'all' })}
          className={`${inputClass} [&>option]:bg-surface`}
        >
          <option value="random">{t("serverSettings:settings.playOneAtRandom")}</option>
          <option value="all">{t("serverSettings:settings.playThemAllIn")}</option>
        </select>
        <span className="flex-1 text-xs text-muted">
          {s.preroll_paths ? (s.preroll_found ? t('serverSettings:settings.prerollFound', { count: s.preroll_found }) : t("serverSettings:settings.noVideosFoundThere")) : t("serverSettings:settings.off")}
        </span>
        {changed && (
          <Button variant="primary" size="sm" disabled={busy} onClick={() => save({ preroll_paths: paths.trim() })}>
            {busy && <Loader2 className="animate-spin" />} {t("serverSettings:settings.save")}</Button>
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

function Section({ sectionKey, title, hint, children }: { sectionKey: string; title: string; hint?: string; children: ReactNode }) {
  const key = sectionKey;
  const group = useContext(SettingsSection);
  if (group !== 'profile' && sectionGroups[key] !== group) return null;
  return (
    <section id={sectionId(key)} className="scroll-mt-24 lg:scroll-mt-6">
      <h2 className="text-lg font-semibold">{title}</h2>
      {hint && <p className="mt-0.5 text-sm text-muted">{hint}</p>}
      <div className="mt-4 divide-y divide-border overflow-hidden rounded-2xl bg-surface ring-1 ring-border">{children}</div>
    </section>
  );
}

type SaveState = { keys: string[]; status: 'idle' | 'pending' | 'error' | 'success'; error: unknown; retry: () => void };
const SaveFeedbackContext = createContext<SaveState | null>(null);

function SaveFeedback({ settings, className }: { settings: string[]; className?: string }) {
  const { t } = useTranslation('serverSettings');
  const save = useContext(SaveFeedbackContext);
  if (!save || !settings.some(key => save.keys.includes(key))) return null;
  if (save.status === 'error') return <div className={className}><ErrorNotice error={save.error} onRetry={save.retry} /></div>;
  if (save.status === 'idle') return null;
  return <p role="status" className={cn('text-xs text-muted', className)}>{save.status === 'pending' ? t("serverSettings:settings.saving") : t("serverSettings:settings.saved")}</p>;
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
  const { t } = useTranslation('serverSettings');
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
            t("serverSettings:settings.setByAnEnvironment")
          ) : current ? (
            <span className="inline-flex items-center gap-1 text-foreground/80">
              <CheckCircle2 className="size-3.5 text-accent" /> {t("serverSettings:settings.key")} {current}
            </span>
          ) : (
            <a href={link} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-accent hover:underline">{t("serverSettings:settings.getAFreeKey")} <ExternalLink className="size-3" />
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
            <input autoFocus value={value} onChange={(e) => setValue(e.target.value)} placeholder={t("serverSettings:settings.pasteTheKey")} className={`${inputClass} w-56`} spellCheck={false} />
            <Button size="sm" variant="primary" type="submit" disabled={!value.trim() || busy}>
              {busy && <Loader2 className="animate-spin" />} {t("serverSettings:settings.save")}</Button>
            <Button size="sm" variant="ghost" type="button" onClick={() => setEditing(false)}>{t("serverSettings:settings.cancel")}</Button>
          </form>
        ) : (
          <div className="flex gap-2">
            {current && (
              <Button size="sm" variant="ghost" onClick={() => save('')} disabled={busy}>{t("serverSettings:settings.remove")}</Button>
            )}
            <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
              {current ? t("serverSettings:settings.change") : t("serverSettings:settings.addKey")}
            </Button>
          </div>
        ))}
    </Row>
  );
}

/** An optional OpenSubtitles account: more downloads a day than the key alone. */
function AccountRow({ username, onSave }: { username: string; onSave: (username: string, password: string) => Promise<unknown> }) {
  const { t } = useTranslation('serverSettings');
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
      label={t("serverSettings:settings.opensubtitlesAccount")}
      description={
        <>
          {username ? (
            <span className="inline-flex items-center gap-1 text-foreground/80">
              <CheckCircle2 className="size-3.5 text-accent" /> {t('serverSettings:settings.accountName', { username })}
            </span>
          ) : (
            t("serverSettings:settings.optionalSigningInAllows")
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
          <input autoFocus value={user} onChange={(e) => setUser(e.target.value)} placeholder={t("serverSettings:settings.username")} autoComplete="off" className={`${inputClass} w-36`} />
          <input value={pass} onChange={(e) => setPass(e.target.value)} type="password" placeholder={t("serverSettings:settings.password")} autoComplete="new-password" className={`${inputClass} w-36`} />
          <Button size="sm" variant="primary" type="submit" disabled={!user.trim() || !pass || busy}>
            {busy && <Loader2 className="animate-spin" />} {t("serverSettings:settings.signIn2")}</Button>
          <Button size="sm" variant="ghost" type="button" onClick={() => setEditing(false)}>{t("serverSettings:settings.cancel")}</Button>
        </form>
      ) : (
        <div className="flex gap-2">
          {username && (
            <Button size="sm" variant="ghost" onClick={() => save('', '')} disabled={busy}>{t("serverSettings:settings.signOut")}</Button>
          )}
          <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
            {username ? t("serverSettings:settings.change") : t("serverSettings:settings.signIn2")}
          </Button>
        </div>
      )}
    </Row>
  );
}

/** Where converted streams are written while they're watched. */
function TranscodeDirRow({ value, onSave }: { value: string; onSave: (v: string) => Promise<unknown> }) {
  const { t } = useTranslation('serverSettings');
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
        label={t("serverSettings:settings.temporaryFolder")}
        description={
          <>{t("serverSettings:settings.whereVideoIsKept")}{' '}
            <span className="font-mono text-foreground/80">{value || t("serverSettings:settings.arcSDataFolder")}</span>
            {error && <span role="alert" className="mt-1 block text-warning">{error}</span>}
          </>
        }
      >
        <div className="flex gap-2">
          {value && (
            <Button size="sm" variant="ghost" onClick={() => void save('')}>{t("serverSettings:settings.useDefault")}</Button>
          )}
          <Button size="sm" variant="secondary" onClick={() => setPicking(true)}>{t("serverSettings:settings.choose")}</Button>
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
  const { t } = useTranslation('accessSettings');
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
      label={t("accessSettings:settings.homeNetworks")}
      description={
        <>{t('accessSettings:settings.networksDescription', { network: tailscale })}
          {!value.includes(tailscale) && (
            <button type="button" className="ml-1 text-accent hover:underline" onClick={() => (setText(value ? `${value}, ${tailscale}` : tailscale), void save(value ? `${value}, ${tailscale}` : tailscale))}>{t("accessSettings:settings.addTailscale")}</button>
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
        <input value={text} onChange={(e) => setText(e.target.value)} placeholder={t("accessSettings:settings.none")} spellCheck={false} className={`${inputClass} w-56 font-mono text-xs`} />
        {text.trim() !== value && (
          <Button size="sm" variant="primary" type="submit">{t("accessSettings:settings.save")}</Button>
        )}
      </form>
    </Row>
  );
}

/** Reverse proxies (SWAG, Nginx Proxy Manager…) whose reports of the visitor's address Arc believes. */
function TrustedProxiesRow({ value, onSave }: { value: string; onSave: (v: string) => Promise<unknown> }) {
  const { t } = useTranslation('accessSettings');
  const [text, setText] = useState(value);
  const [error, setError] = useState('');
  // Show the saved, tidied form (e.g. 172.17.0.5/32) once it's stored.
  useEffect(() => { setText(value); }, [value]);
  return (
    <Row
      label={t('accessSettings:settings.trustedProxies')}
      description={
        <>
          {t('accessSettings:settings.trustedProxiesDescription')}
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
            await onSave(text);
          } catch (err) {
            setError(err instanceof Error ? err.message : String(err));
          }
        }}
      >
        <input aria-label={t('accessSettings:settings.trustedProxies')} value={text} onChange={(e) => setText(e.target.value)} placeholder={t('accessSettings:settings.none')} spellCheck={false} className={`${inputClass} w-56 font-mono text-xs`} />
        {text.trim() !== value && (
          <Button size="sm" variant="primary" type="submit">
            {t('common:actions.save')}
          </Button>
        )}
      </form>
    </Row>
  );
}

function PortRow({ settings: s, onSave }: { settings: ServerSettings; onSave: (port: number) => Promise<unknown> }) {
  const { t } = useTranslation('accessSettings');
  const [port, setPort] = useState(String(s.port));
  const [error, setError] = useState('');
  const [moved, setMoved] = useState('');
  const changed = Number(port) !== s.port;
  const host = s.allow_network ? window.location.hostname : 'localhost';
  return (
    <Row
      label={t("accessSettings:settings.port")}
      description={
        <><Trans ns="accessSettings" i18nKey="settings.localAddress" values={{ address: `http://${host}:${s.port}` }} components={{ address: <span className="font-mono text-foreground/80" /> }} />
          {moved && (
            <span className="mt-1 block text-foreground"><Trans ns="accessSettings" i18nKey="settings.movedAddress" components={{ address: <a className="text-accent hover:underline" href={moved} /> }} />
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
          <Button size="sm" variant="primary" type="submit">{t("accessSettings:settings.change")}</Button>
        )}
      </form>
    </Row>
  );
}

function FFprobeRow({ settings: s, onSave }: { settings: ServerSettings; onSave: (path: string) => Promise<unknown> }) {
  const { t } = useTranslation('serverSettings');
  const [path, setPath] = useState(s.ffprobe_path);
  const [error, setError] = useState('');
  const ok = s.ffprobe.found;
  return (
    <Row
      label={t("serverSettings:settings.ffprobe")}
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
                ? t("serverSettings:settings.ffmpegFoundFilesA")
                : t("serverSettings:settings.ffmpegWasnTFound")}
            </span>
          )}
          {!ok && (
            <a href="https://ffmpeg.org/download.html" target="_blank" rel="noreferrer" className="mt-0.5 inline-flex items-center gap-1 text-accent hover:underline">{t("serverSettings:settings.downloadFfmpeg")} <ExternalLink className="size-3" />
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
        <input value={path} onChange={(e) => setPath(e.target.value)} placeholder={t("serverSettings:settings.foundAutomatically")} className={`${inputClass} min-w-0 flex-1 font-mono text-xs sm:w-72`} spellCheck={false} />
        {path !== s.ffprobe_path && (
          <Button size="sm" variant="primary" type="submit" disabled={s.from_env.ffprobe_path}>{t("serverSettings:settings.save")}</Button>
        )}
      </form>
    </Row>
  );
}

function PasswordRow() {
  const { t } = useTranslation('profileSettings');
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
      label={t("profileSettings:settings.password")}
      description={
        <>{t("profileSettings:settings.usedToSignIn")}{done && <span className="mt-1 block text-foreground">{t("profileSettings:settings.passwordChangedOtherDevices")}</span>}
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
          <input type="password" value={current} onChange={(e) => setCurrent(e.target.value)} placeholder={t("profileSettings:settings.current")} className={`${inputClass} w-36`} autoComplete="current-password" />
          <input type="password" value={next} onChange={(e) => setNext(e.target.value)} placeholder={t("profileSettings:settings.new8Characters")} className={`${inputClass} w-44`} autoComplete="new-password" />
          <Button size="sm" variant="primary" type="submit" disabled={!current || next.length < 8 || change.isPending}>{t("profileSettings:settings.change")}</Button>
        </form>
      ) : (
        <Button size="sm" variant="secondary" onClick={() => setOpen(true)}>{t("profileSettings:settings.changePassword")}</Button>
      )}
    </Row>
  );
}



function processingLabels(): Record<ServerSettings['preview_mode'], string> {
  return { off: i18n.t('serverSettings:processing.off'), scheduled: i18n.t('serverSettings:processing.scheduled'), added: i18n.t('serverSettings:processing.added') };
}

function ProcessingSelect({label,value,busy,onChange}:{label:string;value:ServerSettings['preview_mode'];busy:boolean;onChange:(value:ServerSettings['preview_mode'])=>void}) {
  useTranslation('serverSettings');
 return <select aria-label={label} value={value} disabled={busy} onChange={e=>onChange(e.target.value as ServerSettings['preview_mode'])} className={inputClass+' min-w-0 flex-1 sm:w-72 sm:flex-none [&>option]:bg-surface'}>
  {(Object.keys(processingLabels()) as ServerSettings['preview_mode'][]).map(v => <option key={v} value={v}>{processingLabels()[v]}</option>)}
 </select>;
}
