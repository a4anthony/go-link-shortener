import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { Button } from './ui';

interface ConfirmOptions {
  title: string;
  message?: ReactNode;
  /** Confirm button label. Defaults to "Confirm". */
  confirmLabel?: string;
  /** Cancel button label. Defaults to "Cancel". */
  cancelLabel?: string;
  /** Style the confirm action as destructive. Defaults to true. */
  danger?: boolean;
}

type Confirm = (opts: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<Confirm | null>(null);

interface Pending extends ConfirmOptions {
  resolve: (ok: boolean) => void;
}

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [pending, setPending] = useState<Pending | null>(null);
  const confirmRef = useRef<HTMLButtonElement>(null);

  const confirm = useCallback<Confirm>((opts) => {
    return new Promise<boolean>((resolve) => {
      setPending({ ...opts, resolve });
    });
  }, []);

  const settle = useCallback(
    (ok: boolean) => {
      setPending((p) => {
        p?.resolve(ok);
        return null;
      });
    },
    [],
  );

  // Focus the confirm button on open and wire Escape to cancel.
  useEffect(() => {
    if (!pending) return;
    confirmRef.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') settle(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [pending, settle]);

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {pending && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 px-4"
          onMouseDown={(e) => {
            // Backdrop click cancels; clicks inside the card don't bubble here.
            if (e.target === e.currentTarget) settle(false);
          }}
        >
          <div
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="confirm-title"
            className="w-full max-w-sm rounded-[var(--radius)] border border-border bg-surface shadow-xl shadow-black/50"
          >
            <div className="px-5 py-4">
              <h2 id="confirm-title" className="text-sm font-semibold text-text">
                {pending.title}
              </h2>
              {pending.message && (
                <p className="mt-2 text-sm text-muted">{pending.message}</p>
              )}
            </div>
            <div className="flex justify-end gap-2 border-t border-border px-5 py-3.5">
              <Button variant="ghost" onClick={() => settle(false)}>
                {pending.cancelLabel ?? 'Cancel'}
              </Button>
              <Button
                ref={confirmRef}
                variant={pending.danger === false ? 'primary' : 'danger'}
                onClick={() => settle(true)}
              >
                {pending.confirmLabel ?? 'Confirm'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </ConfirmContext.Provider>
  );
}

export function useConfirm(): Confirm {
  const ctx = useContext(ConfirmContext);
  if (!ctx) throw new Error('useConfirm must be used within ConfirmProvider');
  return ctx;
}
