import { Link } from 'react-router-dom';
import { usingDemoKey } from '../lib/api';

// Global "public sandbox" notice, pinned to the top of the content area on
// every page (mounted once from Layout). Shows only while the console is on the
// seeded demo key — every such visitor authenticates as the same tenant and
// sees the same links; entering a personal key in Settings makes the workspace
// private and hides this. Sticky on md+; static (still top-of-page) on mobile,
// where the header already owns the sticky top slot.
//
// Solid amber strip with dark ink (the same pairing as the primary button) so
// it reads as an unmissable system bar rather than a dismissible inline note.
export function DemoBanner() {
  if (!usingDemoKey()) return null;
  return (
    <div
      role="status"
      className="z-30 border-b px-5 text-sm md:sticky md:top-0 md:px-10"
      style={{ background: 'var(--color-redirect)', borderColor: 'var(--color-accent-dim)', color: '#1a1205' }}
    >
      <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-2 gap-y-0.5 py-2.5">
        <span className="font-semibold">Shared demo playground.</span>
        <span className="opacity-80">
          Links here are public — anyone can view or delete them, and they auto-expire within 24h.
        </span>
        <Link
          to="/going-private"
          className="ml-auto shrink-0 font-medium underline underline-offset-2 hover:opacity-70"
        >
          Make it private →
        </Link>
      </div>
    </div>
  );
}
