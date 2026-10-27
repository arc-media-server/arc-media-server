import i18n from '../i18n';

export type SettingsGroup = 'profile' | 'library' | 'playback' | 'access' | 'maintenance';

export const settingsSections: Record<string, SettingsGroup> = {
  'Online metadata': 'library', 'Ratings': 'library', 'Library scanning': 'library',
  'Moving in': 'library', 'Notifications': 'library',
  'Media tools': 'playback', 'Intro & credit detection': 'playback', 'Pre-roll': 'playback',
  'Subtitle downloads': 'playback', 'Converting': 'playback', 'Optimized Versions': 'playback',
  'Sign-in': 'access', 'Network': 'access', 'Remote Access': 'access', 'Devices': 'access',
  'Scheduled tasks': 'maintenance', 'Diagnostics': 'maintenance',
};

export function settingsNavigation() {
  return [
  { label: i18n.t('common:settings.navigation.personal'), links: [{ id: 'profile', label: i18n.t('common:settings.navigation.yourProfile'), to: '/settings' }] },
  { label: i18n.t('common:settings.navigation.server'), links: [
    { id: 'dashboard', label: i18n.t('common:settings.navigation.dashboard'), to: '/dashboard' },
    { id: 'insights', label: i18n.t('common:settings.navigation.insights'), to: '/insights' },
    { id: 'health', label: i18n.t('common:settings.navigation.libraryHealth'), to: '/health' },
  ] },
  { label: i18n.t('common:settings.navigation.libraries'), links: [
    { id: 'libraries', label: i18n.t('common:settings.navigation.manageLibraries'), to: '/libraries' },
    { id: 'library', label: i18n.t('common:settings.navigation.librariesMetadata'), to: '/settings?section=library' },
  ] },
  { label: i18n.t('common:settings.navigation.playback'), links: [{ id: 'playback', label: i18n.t('common:settings.navigation.playbackTranscoding'), to: '/settings?section=playback' }] },
  { label: i18n.t('common:settings.navigation.peopleAccess'), links: [
    { id: 'profiles', label: i18n.t('common:settings.navigation.profiles'), to: '/profiles' },
    { id: 'access', label: i18n.t('common:settings.navigation.accessDevices'), to: '/settings?section=access' },
  ] },
  { label: i18n.t('common:settings.navigation.maintenance'), links: [
    { id: 'backups', label: i18n.t('common:settings.navigation.backupsRestore'), to: '/backups' },
    { id: 'maintenance', label: i18n.t('common:settings.navigation.maintenanceDiagnostics'), to: '/settings?section=maintenance' },
  ] },
  ];
}

export function isSettingsPath(path: string) {
  return settingsNavigation().some(group => group.links.some(link => link.to.split('?')[0] === path));
}

export function selectedSettingsSection(section: string | null): SettingsGroup {
  return section === 'library' || section === 'playback' || section === 'access' || section === 'maintenance' ? section : 'profile';
}

/** Translate visible section names without changing stored group keys or existing anchors. */
export function settingsSectionTitle(title: string): string {
  const labels: Record<string, string> = {
    "Playback": i18n.t('common:settings.sections.playback'),
    "Subtitles": i18n.t('common:settings.sections.subtitles'),
    "Display": i18n.t('common:settings.sections.display'),
    "Online metadata": i18n.t('common:settings.sections.onlineMetadata'),
    "Ratings": i18n.t('common:settings.sections.ratings'),
    "Library scanning": i18n.t('common:settings.sections.libraryScanning'),
    "Moving in": i18n.t('common:settings.sections.movingIn'),
    "Notifications": i18n.t('common:settings.sections.notifications'),
    "Media tools": i18n.t('common:settings.sections.mediaTools'),
    "Intro & credit detection": i18n.t('common:settings.sections.introCreditDetection'),
    "Pre-roll": i18n.t('common:settings.sections.preRoll'),
    "Subtitle downloads": i18n.t('common:settings.sections.subtitleDownloads'),
    "Converting": i18n.t('common:settings.sections.converting'),
    "Optimized Versions": i18n.t('common:settings.sections.optimizedVersions'),
    "Sign-in": i18n.t('common:settings.sections.signIn'),
    "Network": i18n.t('common:settings.sections.network'),
    "Remote Access": i18n.t('common:settings.sections.remoteAccess'),
    "Devices": i18n.t('common:settings.sections.devices'),
    "Scheduled tasks": i18n.t('common:settings.sections.scheduledTasks'),
    "Diagnostics": i18n.t('common:settings.sections.diagnostics'),
  };
  return labels[title] ?? title;
}

/** Media tools (ffprobe/FFmpeg) is shown only when something needs attention or a path was set:
 * when both are found automatically there is nothing to do there. */
export function mediaToolsNeeded(s?: { ffprobe_path: string; ffprobe: { found: boolean; ffmpeg?: string } }) {
  return !!s && (!s.ffprobe.found || !s.ffprobe.ffmpeg || !!s.ffprobe_path);
}
