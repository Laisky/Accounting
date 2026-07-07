import { useBook } from '@/contexts/BookContext';
import { ReportWorkspace } from '@/features/reports/ReportWorkspace';
import { useShellChrome } from '@/features/shell/useShellChrome';

// ReportsRoute supplies the routed report dimension and display currency to the report workspace.
function ReportsRoute() {
  const { reportTab } = useShellChrome();
  const { displayCurrency } = useBook();

  return <ReportWorkspace activeTab={reportTab} baseCurrency={displayCurrency} />;
}

export default ReportsRoute;
