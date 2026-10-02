import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { errorMessage } from '../api/client';

export function Button({
  variant = 'primary',
  className = '',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: 'primary' | 'secondary' | 'quiet' }) {
  return <button {...props} className={`button ${variant} ${className}`} />;
}
export function Panel({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <section className={`panel ${className}`}>{children}</section>;
}
export function Loading() {
  return (
    <div className="loading" role="status">
      <span className="spinner" />
      Загружаем коллекцию…
    </div>
  );
}
export function ErrorNotice({ error, retry }: { error: unknown; retry?: () => void }) {
  return (
    <div className="error-notice" role="alert">
      <p>{errorMessage(error)}</p>
      {retry && (
        <Button variant="secondary" onClick={retry}>
          Повторить
        </Button>
      )}
    </div>
  );
}
export function Empty({
  title,
  children,
  action,
}: {
  title: string;
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <Panel className="empty">
      <span className="empty-symbol" aria-hidden="true">
        ✧
      </span>
      <h2>{title}</h2>
      <p>{children}</p>
      {action}
    </Panel>
  );
}
