export const colors = {
  blue: '#1754EB', navy: '#182235', blueTint: '#DCE6FD', blueSoft: '#9DB8F7', onBlueMuted: '#C9D8FD',
  white: '#FFFFFF', textMuted: '#5A6478', surface: '#F3F6FB', surfaceAlt: '#F7F8FB', key: '#EEF1F5',
  keyText: '#4A5263', segActive: '#F1F3F7', softButton: '#EEF2F7', divider: '#EEF2F7', grabber: '#D5DCE8',
  dotted: '#9AA3B5', scrim: 'rgba(24,34,53,0.5)', green: '#36C38F', greenText: '#0E7A55',
  greenTint: '#D7F3E8', orange: '#C46A0C', orangeText: '#8A4A06', orangeTint: '#FBEBDA', red: '#DD4A55',
  redText: '#C2323D', redTint: '#FBE1E3',
  // Compatibility aliases used by the onboarding flow.
  background: '#FFFFFF', ink: '#182235', primary: '#1754EB', muted: '#69738B', border: '#182235', error: '#C2323D',
} as const;

export const radii = { tag: 6, sm: 8, md: 12, lg: 16, keypad: 32, sheet: 28, pill: 999 } as const;
export const spacing = { xs: 4, sm: 8, md: 12, lg: 16, xl: 20, xxl: 24, screenX: 16, sheetX: 18 } as const;
export const layout = { homeHeaderHeight: 342, sheetHeaderHeight: 160, sheetOverlap: 28, actionButtonHeight: 60, buttonHeight: 56, keyHeight: 56, segHeight: 44, inputHeight: 44, touchMin: 44 } as const;
export const fonts = { regular: 'Satoshi-Regular', medium: 'Satoshi-Medium', bold: 'Satoshi-Bold', black: 'Satoshi-Black' } as const;
export const type = {
  amount: { fontFamily: fonts.bold, fontSize: 60, letterSpacing: -2 }, balance: { fontFamily: fonts.bold, fontSize: 44, letterSpacing: -1.5 },
  title: { fontFamily: fonts.bold, fontSize: 28, letterSpacing: -0.6 }, welcome: { fontFamily: fonts.bold, fontSize: 26, letterSpacing: -0.5 },
  menuItem: { fontFamily: fonts.bold, fontSize: 24, letterSpacing: -0.4 }, tab: { fontFamily: fonts.bold, fontSize: 18 },
  button: { fontFamily: fonts.bold, fontSize: 17 }, bodyStrong: { fontFamily: fonts.bold, fontSize: 15 },
  body: { fontFamily: fonts.medium, fontSize: 15, lineHeight: 22 }, caption: { fontFamily: fonts.medium, fontSize: 13 },
  hint: { fontFamily: fonts.medium, fontSize: 15 }, kicker: { fontFamily: fonts.bold, fontSize: 11, letterSpacing: 1.6, textTransform: 'uppercase' as const },
  key: { fontFamily: fonts.bold, fontSize: 26 }, operator: { fontFamily: fonts.medium, fontSize: 28 },
} as const;
export const swipe = { confirmThreshold: 0.6, maxTranslate: 48, hapticOnFire: true } as const;
