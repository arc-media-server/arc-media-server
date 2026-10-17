import { useEffect } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router';
import { Compass } from 'lucide-react';
import { MobileBar, Sidebar } from './components/Sidebar';
import { PlayerProvider } from './components/Player';
import { EmptyState, Page } from './components/Page';
import { Home } from './pages/Home';
import { LibraryPage } from './pages/Library';
import { MoviePage } from './pages/Movie';
import { SeriesPage } from './pages/Series';
import { SearchPage } from './pages/Search';
import { HealthPage } from './pages/Health';
import { BackupsPage } from './pages/Backups';
import { DashboardPage } from './pages/Dashboard';
import { CollectionPage } from './pages/Collection';
import { PlaylistPage, PlaylistsPage } from './pages/Playlists';
import { LibrariesPage } from './pages/Libraries';
import { SettingsPage } from './pages/Settings';
import { ProfilesPage } from './pages/Profiles';
import { AuthGate } from './components/auth/AuthGate';
import { useProfileTheme } from './lib/theme';

export function App() {
  const { pathname } = useLocation();
  // Braces matter: scrollTo returns a promise in newer browsers, which React would take as a cleanup.
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);

  return (
    <AuthGate>
      <PlayerProvider>
        <ProfileTheme />
        <Sidebar />
        <MobileBar />
        <main className="min-h-dvh md:pl-[248px]">
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/library/:id" element={<LibraryPage />} />
            <Route path="/movie/:id" element={<MoviePage />} />
            <Route path="/series/:id" element={<SeriesPage />} />
            <Route path="/collection/:id" element={<CollectionPage />} />
            <Route path="/playlists" element={<PlaylistsPage />} />
            <Route path="/playlist/:id" element={<PlaylistPage />} />
            <Route path="/search" element={<SearchPage />} />
            <Route path="/health" element={<HealthPage />} />
            <Route path="/attention" element={<Navigate to="/health" replace />} />
            <Route path="/backups" element={<BackupsPage />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/libraries" element={<LibrariesPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/profiles" element={<ProfilesPage />} />
            <Route
              path="*"
              element={
                <Page>
                  <EmptyState icon={<Compass />} title="Nothing here">
                    This page doesn't exist.
                  </EmptyState>
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
