export type SettingsGroup = 'profile' | 'library' | 'playback' | 'access' | 'maintenance';

export const settingsSections: Record<string, SettingsGroup> = {
  'Online metadata': 'library', 'Ratings': 'library', 'Library scanning': 'library',
  'Moving in': 'library', 'Notifications': 'library',
  'Media tools': 'playback', 'Intro & credit detection': 'playback', 'Pre-roll': 'playback',
  'Subtitle downloads': 'playback', 'Converting': 'playback', 'Optimized Versions': 'playback',
  'Sign-in': 'access', 'Network': 'access', 'Remote Access': 'access', 'Devices': 'access',
  'Scheduled tasks': 'maintenance', 'Diagnostics': 'maintenance',
};

export const settingsNavigation = [
  { label: 'Personal', links: [{ id: 'profile', label: 'Your profile', to: '/settings' }] },
  { label: 'Server', links: [
    { id: 'dashboard', label: 'Dashboard', to: '/dashboard' },
    { id: 'insights', label: 'Insights', to: '/insights' },
    { id: 'health', label: 'Library Health', to: '/health' },
  ] },
  { label: 'Libraries', links: [
    { id: 'libraries', label: 'Manage libraries', to: '/libraries' },
    { id: 'library', label: 'Libraries & metadata', to: '/settings?section=library' },
  ] },
  { label: 'Playback', links: [{ id: 'playback', label: 'Playback & transcoding', to: '/settings?section=playback' }] },
  { label: 'People & access', links: [
    { id: 'profiles', label: 'Profiles', to: '/profiles' },
    { id: 'access', label: 'Access & devices', to: '/settings?section=access' },
  ] },
  { label: 'Maintenance', links: [
    { id: 'backups', label: 'Backups & restore', to: '/backups' },
    { id: 'maintenance', label: 'Maintenance & diagnostics', to: '/settings?section=maintenance' },
  ] },
];

export function isSettingsPath(path: string) {
  return settingsNavigation.some(group => group.links.some(link => link.to.split('?')[0] === path));
}

export function selectedSettingsSection(section: string | null): SettingsGroup {
  return section === 'library' || section === 'playback' || section === 'access' || section === 'maintenance' ? section : 'profile';
}
