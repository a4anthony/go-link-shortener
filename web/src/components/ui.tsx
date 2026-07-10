import { forwardRef } from 'react';
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from 'react';

type ButtonVariant = 'primary' | 'ghost' | 'danger' | 'subtle';

const buttonVariants: Record<ButtonVariant, string> = {
  primary:
    'bg-accent text-[#1a1205] hover:bg-[color-mix(in_srgb,var(--color-accent)_88%,white)] font-semibold',
  subtle: 'bg-raised text-text border border-border hover:border-border-strong',
  ghost: 'bg-transparent text-muted border border-border hover:text-text hover:border-border-strong',
  danger:
    'bg-transparent text-error border border-[color-mix(in_srgb,var(--color-error)_40%,transparent)] hover:bg-[color-mix(in_srgb,var(--color-error)_12%,transparent)]',
};

export const Button = forwardRef<
  HTMLButtonElement,
  ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }
>(function Button({ variant = 'subtle', className = '', children, ...props }, ref) {
  return (
    <button
      ref={ref}
      className={`inline-flex items-center justify-center gap-2 rounded-[var(--radius)] px-3.5 py-2 text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${buttonVariants[variant]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
});

export function Input({ className = '', ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={`w-full rounded-[var(--radius)] border border-border bg-bg px-3 py-2 text-sm text-text placeholder:text-faint focus:border-accent focus:outline-none ${className}`}
      {...props}
    />
  );
}

export function Select({ className = '', children, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={`w-full rounded-[var(--radius)] border border-border bg-bg px-3 py-2 text-sm text-text focus:border-accent focus:outline-none ${className}`}
      {...props}
    >
      {children}
    </select>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-xs font-medium uppercase tracking-wide text-muted">{label}</span>
      {children}
      {hint && <span className="text-xs text-faint">{hint}</span>}
    </label>
  );
}

export function Panel({ className = '', children }: { className?: string; children: ReactNode }) {
  return (
    <div className={`rounded-[var(--radius)] border border-border bg-surface ${className}`}>
      {children}
    </div>
  );
}

export function PanelHeader({ title, action }: { title: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between border-b border-border px-5 py-3.5">
      <h2 className="text-sm font-semibold text-text">{title}</h2>
      {action}
    </div>
  );
}

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 text-sm text-muted">
      <span
        aria-hidden
        className="h-4 w-4 animate-spin rounded-full border-2 border-border border-t-accent"
      />
      {label ?? 'Loading…'}
    </div>
  );
}

export function EmptyState({
  title,
  children,
  action,
}: {
  title: string;
  children?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-6 py-14 text-center">
      <div className="font-mono text-2xl text-faint">{'{ }'}</div>
      <h3 className="text-base font-semibold text-text">{title}</h3>
      {children && <p className="max-w-sm text-sm text-muted">{children}</p>}
      {action}
    </div>
  );
}

export function ErrorNote({ message }: { message: string }) {
  return (
    <div
      className="rounded-[var(--radius)] border px-4 py-3 text-sm"
      style={{
        borderColor: 'color-mix(in srgb, var(--color-error) 40%, transparent)',
        background: 'color-mix(in srgb, var(--color-error) 8%, transparent)',
        color: 'var(--color-error)',
      }}
    >
      {message}
    </div>
  );
}
