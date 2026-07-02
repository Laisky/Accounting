import auth from './auth.json';
import common from './common.json';
import imports from './imports.json';
import ledger from './ledger.json';
import mobile from './mobile.json';
import reports from './reports.json';

const translations = {
  ...common,
  ...auth,
  ...ledger,
  ...imports,
  ...reports,
  ...mobile,
};

export default translations;
