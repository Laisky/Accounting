import api from './api.json';
import auth from './auth.json';
import common from './common.json';
import imports from './imports.json';
import landing from './landing.json';
import ledger from './ledger.json';
import mobile from './mobile.json';
import reports from './reports.json';

const translations = {
  ...common,
  ...api,
  ...auth,
  ...landing,
  ...ledger,
  ...imports,
  ...reports,
  ...mobile,
};

export default translations;
