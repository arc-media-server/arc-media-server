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
  { label: i18n.t('common:settings.navigation.personal'), links: [{ id: 'profile', label: i18n.t('common:settings.navigation.yourprofile'), to: '/settings' }] },
  { label: i18n.t('common:settings.navigation.server'), links: [
    { id: 'dashboard', label: i18n.t('common:settings.navigation.dashboard'), to: '/dashboard' },
    { id: 'insights', label: i18n.t('common:settings.navigation.insights'), to: '/insights' },
    { id: 'health', label: i18n.t('common:settings.navigation.libraryHealth'), to: '/health' },
  ] },
  { label: i18n.t('common:settings.navigation.libraries'), links: [
    { id: 'libraries', label: i18n.t('common:settings.navigation.managelibraries'), to: '/libraries' },
    { id: 'library', label: i18n.t('common:settings.navigation.librariesmetadata'), to: '/settings?section=library' },
  ] },
  { label: i18n.t('common:settings.navigation.playback'), links: [{ id: 'playback', label: i18n.t('common:settings.navigation.playbacktranscoding'), to: '/settings?section=playback' }] },
  { label: i18n.t('common:settings.navigation.peopleaccess'), links: [
    { id: 'profiles', label: i18n.t('common:settings.navigation.profiles'), to: '/profiles' },
    { id: 'access', label: i18n.t('common:settings.navigation.accessdevices'), to: '/settings?section=access' },
  ] },
  { label: i18n.t('common:settings.navigation.maintenance'), links: [
    { id: 'backups', label: i18n.t('common:settings.navigation.backupsrestore'), to: '/backups' },
    { id: 'maintenance', label: i18n.t('common:settings.navigation.maintenancediagnostics'), to: '/settings?section=maintenance' },
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
    "Online metadata": i18n.t('common:settings.sections.onlinemetadata'),
    "Ratings": i18n.t('common:settings.sections.ratings'),
    "Library scanning": i18n.t('common:settings.sections.libraryscanning'),
    "Moving in": i18n.t('common:settings.sections.movingin'),
    "Notifications": i18n.t('common:settings.sections.notifications'),
    "Media tools": i18n.t('common:settings.sections.mediatools'),
    "Intro & credit detection": i18n.t('common:settings.sections.introcreditdetection'),
    "Pre-roll": i18n.t('common:settings.sections.preroll'),
    "Subtitle downloads": i18n.t('common:settings.sections.subtitledownloads'),
    "Converting": i18n.t('common:settings.sections.converting'),
    "Optimized Versions": i18n.t('common:settings.sections.optimizedVersions'),
    "Sign-in": i18n.t('common:settings.sections.signin'),
    "Network": i18n.t('common:settings.sections.network'),
    "Remote Access": i18n.t('common:settings.sections.remoteAccess'),
    "Devices": i18n.t('common:settings.sections.devices'),
    "Scheduled tasks": i18n.t('common:settings.sections.scheduledtasks'),
    "Diagnostics": i18n.t('common:settings.sections.diagnostics'),
  };
  return labels[title] ?? title;
}
