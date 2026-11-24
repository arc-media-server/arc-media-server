import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { api, ApiError, DATABASE_UNAVAILABLE_EVENT } from '@/lib/api';
import { AuthScreen, Card } from './Screen';

export function useDatabaseOutage(error: Error | null) {
  const qc = useQueryClient();
  const [sawOutage, setSawOutage] = useState(false);
  const offline = sawOutage || (error instanceof ApiError && error.code === 'database_unavailable');
  useEffect(() => {
    const unavailable = () => setSawOutage(true);
    window.addEventListener(DATABASE_UNAVAILABLE_EVENT, unavailable);
    return () => window.removeEventListener(DATABASE_UNAVAILABLE_EVENT, unavailable);
  }, []);
  useEffect(() => {
    if (!offline) return;
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const retry = async () => {
      try {
        const status = await api.authStatus();
        if (!active) return;
        qc.setQueryData(['auth'], status);
        setSawOutage(false);
        void qc.invalidateQueries({ predicate: query => query.queryKey[0] !== 'auth' });
      } catch {
        if (active) timer = setTimeout(retry, 3000);
      }
    };
    timer = setTimeout(retry, 3000);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [offline, qc]);
  return offline;
}

export function DatabaseOutage({ fullPage }: { fullPage?: boolean }) {
  const { t } = useTranslation('auth');
  if (fullPage) {
    return <AuthScreen><Card><div role="status">
      <h1 className="text-2xl font-bold">{t('databaseOutage.heading')}</h1>
      <p className="mt-1.5 text-sm text-muted">{t('databaseOutage.retrying')}</p>
    </div></Card></AuthScreen>;
  }
  return <div role="status" className="fixed left-1/2 top-3 z-[60] w-max max-w-[calc(100vw-2rem)] -translate-x-1/2 rounded-xl bg-surface px-4 py-3 text-sm text-foreground shadow-xl ring-1 ring-border">
    <span className="font-semibold">{t('databaseOutage.heading')}</span>
    <p className="text-muted">{t('databaseOutage.retrying')}</p>
  </div>;
}
