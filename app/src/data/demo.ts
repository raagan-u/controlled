import type { Activity } from './activity';

export type { Activity } from './activity';
export const activities: Activity[] = [
  { amount: '24.00', id: 'txn-001', label: 'Riya Shah', meta: 'Sent · Dinner split', outgoing: true, section: 'Today', status: 'Completed' },
  { amount: '200.00', id: 'txn-002', label: 'Withdraw to PayPal', meta: 'Pending', outgoing: true, section: 'Today', status: 'Pending' },
  { amount: '40.00', id: 'txn-003', label: 'Kabir Rao', meta: 'Requested · Concert tickets', requested: true, section: 'Today', status: 'Completed' },
  { amount: '40.00', id: 'txn-004', label: 'Arjun Mehta', meta: 'Received · Cab', section: 'Yesterday', status: 'Completed' },
  { amount: '500.00', id: 'txn-005', label: 'Added from PayPal', meta: 'alex@example.com', section: 'Yesterday', status: 'Completed' },
  { amount: '12.50', id: 'txn-006', label: 'Meera Iyer', meta: 'Sent · Coffee', outgoing: true, section: 'Oct 27', status: 'Completed' },
  { amount: '100.00', id: 'txn-007', label: 'Added from PayPal', meta: 'Failed', section: 'Oct 27', status: 'Failed' },
];
export const contacts = [
  { initials: 'RS', name: 'Riya Shah', username: '@riya' }, { initials: 'AM', name: 'Arjun Mehta', username: '@arjun' },
  { initials: 'MI', name: 'Meera Iyer', username: '@meera' }, { initials: 'KR', name: 'Kabir Rao', username: '@kabir' },
];
