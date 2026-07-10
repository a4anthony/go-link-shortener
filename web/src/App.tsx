import { lazy, Suspense } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import { Layout } from './components/Layout';
import { Spinner } from './components/ui';

// Route-level code splitting keeps the initial bundle small; the charting
// library only loads when the link detail page is opened.
const Overview = lazy(() => import('./pages/Overview').then((m) => ({ default: m.Overview })));
const Links = lazy(() => import('./pages/Links').then((m) => ({ default: m.Links })));
const LinkDetail = lazy(() => import('./pages/LinkDetail').then((m) => ({ default: m.LinkDetail })));
const Webhooks = lazy(() => import('./pages/Webhooks').then((m) => ({ default: m.Webhooks })));
const Settings = lazy(() => import('./pages/Settings').then((m) => ({ default: m.Settings })));
const GoingPrivate = lazy(() => import('./pages/GoingPrivate').then((m) => ({ default: m.GoingPrivate })));

export default function App() {
  return (
    <Layout>
      <Suspense fallback={<Spinner label="Loading…" />}>
        <Routes>
          <Route path="/" element={<Overview />} />
          <Route path="/links" element={<Links />} />
          <Route path="/links/:id" element={<LinkDetail />} />
          <Route path="/webhooks" element={<Webhooks />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/going-private" element={<GoingPrivate />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Suspense>
    </Layout>
  );
}
