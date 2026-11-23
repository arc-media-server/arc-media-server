import { useTranslation } from 'react-i18next';
import { fieldClass, Label } from './Screen';

export type PostgresFieldsValue = {
  host: string;
  port: string;
  database: string;
  user: string;
  password: string;
};

export function PostgresFields({ fields, setFields, noTLS, setNoTLS }: {
  fields: PostgresFieldsValue;
  setFields: (fields: PostgresFieldsValue) => void;
  noTLS: boolean;
  setNoTLS: (noTLS: boolean) => void;
}) {
  const { t } = useTranslation('auth');
  const labels = {
    host: t('auth:database.host'), port: t('auth:database.port'), database: t('auth:database.database'),
    user: t('auth:database.user'), password: t('auth:database.password'),
  };
  return (
    <>
      {(['host', 'port', 'database', 'user', 'password'] as const).map((field) => (
        <label key={field} className="block">
          <Label>{labels[field]}</Label>
          <input required className={fieldClass} value={fields[field]} onChange={(event) => setFields({ ...fields, [field]: event.target.value })}
            type={field === 'port' ? 'number' : field === 'password' ? 'password' : 'text'} min={field === 'port' ? 1 : undefined}
            max={field === 'port' ? 65535 : undefined} autoComplete={field === 'password' ? 'new-password' : 'off'} spellCheck={false} />
        </label>
      ))}
      <label className="flex items-start gap-3 text-sm">
        <input className="mt-1" type="checkbox" checked={noTLS} onChange={(event) => setNoTLS(event.target.checked)} />
        {t('auth:database.noTls')}
      </label>
    </>
  );
}
