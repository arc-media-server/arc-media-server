# Official provider logos

These are the original bytes downloaded on 12 November 2026 from the providers' own
attribution pages. They are bundled with the app: viewing a credit makes no request to a
provider. Static imports in ProviderCredits.tsx make a missing asset a build error.

| File | Official source | SHA-256 |
| --- | --- | --- |
| `tmdb.svg` | [TMDB approved Alt short (blue)](https://www.themoviedb.org/assets/v4/logos/v2/blue_short-8e7b30f73a4020692ccca9c88bafe5dcb6f8a62a4c6bc55cd9ba82bb2cd95f6c.svg) | `8e7b30f73a4020692ccca9c88bafe5dcb6f8a62a4c6bc55cd9ba82bb2cd95f6c` |
| `thetvdb-light.png` | [TheTVDB logo2](https://thetvdb.com/images/attribution/logo2.png) | `e519be4f7968bf1740a85404290632596da6942190f84527588e53ec8e59fada` |
| `thetvdb-dark.png` | [TheTVDB logo1](https://thetvdb.com/images/attribution/logo1.png) | `9edbcc405191b5c687bde857aa2f9a1841f85404eeda39964ccc8e5af03100a5` |

Requirements and asset listings:
- [TMDB FAQ](https://developer.themoviedb.org/docs/faq) and
  [approved logos](https://www.themoviedb.org/about/logos-attribution): approved logo and
  non-endorsement notice in About/Credits; mark less prominent than Arc; link to its homepage.
- [TheTVDB attribution](https://thetvdb.com/api-information): direct attribution link for end
  users viewing its API metadata, unless an alternative is approved. That page supplies these
  two logo variants; its attribution text does not separately mandate a logo.

Never recolour, stretch, crop, rotate, filter or redraw these marks. Object containment and
bounded display sizes preserve the supplied aspect ratios. TheTVDB variants follow the same
profile/system light/dark rules as Arc's theme tokens. Provider marks retain their own colours.

Provider marks belong to their respective owners; Arc's source licence does not grant rights
to them. These are attribution assets, not an endorsement or a provider distribution licence.
TheTVDB's distribution/access agreement remains an owner decision in #80.
