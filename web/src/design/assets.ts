// Single source of truth for replaceable artwork.
// Swapping an image never requires changing buttons, page routing or application data.
export const designAssets = {
  hero: {
    modern: '/assets/illustrations/hero-city.avif',
    fallback: '/assets/approved-city.webp',
  },
  contours: '/assets/backgrounds/contours.svg',
  waves: '/assets/backgrounds/waves.svg',
  shortcutBuilding: '/assets/illustrations/shortcut-building.svg',
  shortcutFavorite: '/assets/illustrations/shortcut-favorite.svg',
  profileReferralGift: '/assets/illustrations/profile-referral-gift.svg',
} as const;
