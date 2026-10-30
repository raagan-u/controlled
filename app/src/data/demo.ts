export type Activity = { amount: string; id: string; label: string; meta: string; outgoing?: boolean; status: 'Completed' | 'Pending' | 'Failed' };
export const activities: Activity[] = [
  { amount: '42.00', id: 'txn-001', label: 'Dinner with Maya', meta: 'Today, 8:14 PM', outgoing: true, status: 'Completed' },
  { amount: '250.00', id: 'txn-002', label: 'Added money', meta: 'Yesterday, 2:30 PM', status: 'Completed' },
  { amount: '18.50', id: 'txn-003', label: 'Coffee from Noah', meta: 'Oct 28, 10:02 AM', status: 'Completed' },
  { amount: '75.00', id: 'txn-004', label: 'Withdrawal', meta: 'Oct 27, 4:41 PM', outgoing: true, status: 'Pending' },
];
export const contacts = [
  { initials: 'MK', name: 'Maya Kapoor', username: '@maya' }, { initials: 'NO', name: 'Noah Oliveira', username: '@noah' },
  { initials: 'AR', name: 'Ari Rao', username: '@ari' }, { initials: 'SL', name: 'Sam Lee', username: '@sam' },
];
