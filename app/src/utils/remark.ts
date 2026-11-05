export function isMeaningfulRemark(value: string) {
  const normalized = value.trim().toLowerCase();
  if (normalized.length < 2) return false;
  if (['asdf', 'qwer', 'zxcv', 'hjkl', '1234'].some((noise) => normalized.includes(noise))) return false;
  if (/(.)\1{3,}/.test(normalized)) return false;
  return (normalized.match(/[a-z]/g) ?? []).length >= 2;
}
