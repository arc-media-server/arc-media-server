import { useTranslation } from 'react-i18next';
import { useEffect } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router';
import { Compass } from 'lucide-react';
import { MobileBar, Sidebar } from './components/Sidebar';
import { PlayerProvider } from './components/Player';
import { MatchUpdates } from './components/FixMatch';
import { EmptyState, Page } from './components/Page';
import { Home } from './pages/Home';
import { LibraryPage } from './pages/Library';
import { MoviePage } from './pages/Movie';
import { SeriesPage } from './pages/Series';
import { SearchPage } from './pages/Search';
import { HealthPage } from './pages/Health';
import { BackupsPage } from './pages/Backups';
import { DashboardPage } from './pages/Dashboard';
import { InsightsPage } from './pages/Insights';
import { WatchlistPage } from './pages/Watchlist';
import { YearPage } from './pages/Year';
import { CollectionPage } from './pages/Collection';
import { PlaylistPage, PlaylistsPage } from './pages/Playlists';
import { PersonPage } from './pages/Person';
import { LibrariesPage } from './pages/Libraries';
import { SettingsPage } from './pages/Settings';
import { SettingsArea } from './components/SettingsArea';
import { ProfilesPage } from './pages/Profiles';
import { LinkDevicePage } from './components/Devices';
import { AuthGate } from './components/auth/AuthGate';
import { useProfileTheme } from './lib/theme';
import { useLiveUpdates } from './lib/live-updates';
import { useAuth } from './lib/auth';

export function App() {
  const { t } = useTranslation('common');
  const { pathname } = useLocation();
  // Braces matter: scrollTo returns a promise in newer browsers, which React would take as a cleanup.
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);

  return (
    <AuthGate>
      <PlayerProvider>
        <ProfileTheme />
        <LiveUpdates />
        <Sidebar />
        <MobileBar />
        <MatchUpdates />
        <main className="min-h-dvh md:pl-[248px]">
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/library/:id" element={<LibraryPage />} />
            <Route path="/movie/:id" element={<MoviePage />} />
            <Route path="/series/:id" element={<SeriesPage />} />
            <Route path="/collection/:id" element={<CollectionPage />} />
            <Route path="/playlists" element={<PlaylistsPage />} />
            <Route path="/watchlist" element={<WatchlistPage />} />
            <Route path="/person/:id" element={<PersonPage />} />
            <Route path="/playlist/:id" element={<PlaylistPage />} />
            <Route path="/search" element={<SearchPage />} />
            <Route element={<SettingsArea />}>
              <Route path="/health" element={<HealthPage />} />
              <Route path="/backups" element={<BackupsPage />} />
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/insights" element={<InsightsPage />} />
              <Route path="/libraries" element={<LibrariesPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/profiles" element={<ProfilesPage />} />
            </Route>
            <Route path="/attention" element={<Navigate to="/health" replace />} />
            <Route path="/year" element={<YearPage />} />
            <Route path="/link" element={<LinkDevicePage />} />
            <Route
              path="*"
              element={
                <Page>
                  <EmptyState icon={<Compass />} title={t('errors.notFoundHeading')}>{t('errors.notFound')}</EmptyState>
                </Page>
              }
            />
          </Routes>
        </main>
      </PlayerProvider>
    </AuthGate>
  );
}

/** Keeps the page in the signed-in profile's chosen theme. */
function ProfileTheme() {
  useProfileTheme();
  return null;
}

/** Refreshes what's on screen when Arc says something changed (scans, other devices, artwork). */
function LiveUpdates() {
  useLiveUpdates(useAuth().data?.profile?.id);
  return null;
}
