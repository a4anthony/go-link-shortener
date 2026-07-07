import { Link } from 'react-router-dom';
import { PageHeader } from '../components/Layout';
import { Stat } from '../components/Stat';
import { Bars } from '../components/data';
import { Button, ErrorNote, Panel, PanelHeader, Spinner } from '../components/ui';
import { useAsync } from '../hooks/useAsync';
import { api } from '../lib/api';
import { formatNumber } from '../lib/format';

export function Overview() {
  const { data, error, loading } = useAsync(() => api.overview(), []);

  return (
    <>
      <PageHeader
        title="Overview"
        subtitle="Traffic across every link in this tenant."
        action={
          <Link to="/links">
            <Button variant="primary">New link</Button>
          </Link>
        }
      />

      {loading && <Spinner label="Loading overview…" />}
      {error && <ErrorNote message={error} />}

      {data && (
        <div className="flex flex-col gap-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Stat label="Total links" value={formatNumber(data.total_links)} />
            <Stat label="Total clicks" value={formatNumber(data.total_clicks)} accent />
            <Stat
              label="Avg clicks / link"
              value={
                data.total_links > 0
                  ? (data.total_clicks / data.total_links).toFixed(1)
                  : '0'
              }
            />
          </div>

          <Panel>
            <PanelHeader title="Busiest links" />
            <div className="p-5">
              {data.top_links && data.top_links.length > 0 ? (
                <Bars data={data.top_links} empty="No clicks yet." />
              ) : (
                <div className="py-8 text-center text-sm text-muted">
                  No clicks recorded yet.{' '}
                  <Link to="/links" className="text-accent hover:underline">
                    Create a link
                  </Link>{' '}
                  and share it.
                </div>
              )}
            </div>
          </Panel>
        </div>
      )}
    </>
  );
}
