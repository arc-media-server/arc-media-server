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
import { LibrariesPage } from './pages/Libraries';
import { SettingsPage } from './pages/Settings';
import { ProfilesPage } from './pages/Profiles';
import { AuthGate } from './components/auth/AuthGate';

export function App() {
  const { pathname } = useLocation();
  // Braces matter: scrollTo returns a promise in newer browsers, which React would take as a cleanup.
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);

  return (
    <AuthGate>
      <PlayerProvider>
        <Sidebar />
        <MobileBar />
        <main className="min-h-dvh md:pl-[248px]">
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/library/:id" element={<LibraryPage />} />
            <Route path="/movie/:id" element={<MoviePage />} />
            <Route path="/series/:id" element={<SeriesPage />} />
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
