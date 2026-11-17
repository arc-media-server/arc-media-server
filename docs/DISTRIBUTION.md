# Provider requirements before public distribution

Attribution rechecked 12 November 2026; distribution-model investigation dated 21 October 2026.
This records published requirements and unresolved project decisions; it does not establish
that Arc has a provider licence or approval.

## Attribution (#64)

TMDB requires an approved, unmodified logo and its non-endorsement notice in an About/Credits
section. Its logo must be less prominent than Arc's branding. See the official
[TMDB FAQ](https://developer.themoviedb.org/docs/faq), including its approved-logo link.

TheTVDB requires a direct attribution link visible to users viewing its metadata, unless it
has approved another arrangement. Its [API information](https://thetvdb.com/api-information)
page provides sample wording and brand assets. Arc bundles the original official marks in
`web/src/assets/attribution/` (source URLs and hashes in its README), displays them in every
profile's About/Credits and on matched movie/show pages, and links directly to the providers.
TheTVDB light/dark variants follow the profile/system theme without recolouring. The TMDB
notice is retained verbatim; it does not imply all libraries use that provider. This implements
#64's web attribution, not approval to distribute provider data or keys. Native clients must
meet their own visible-attribution requirements.

## TheTVDB distribution model (#65)

The current [API information](https://thetvdb.com/api-information) page describes licensing
tiers, including an attribution-based free tier for projects below its stated revenue threshold.
That page alone does not show that Arc's existing private-use key is approved for distribution.

The official [v4 application guidance](https://support.thetvdb.com/kb/faq.php?id=81) says keys
are applied for by projects, rather than issued to individual end users of those projects.
It describes negotiated-project access and subscriber-supported access, with a distinct
subscriber PIN sent alongside the project key. Its guidance is older than the current pricing
page, so Arc's exact arrangement must be confirmed with TheTVDB before release.

Recommended next decision: seek an Arc-specific project approval covering the intended
open-source distribution and access pattern. Keep existing private-use credentials private;
do not embed the owner's key in source or an image. If subscriber-supported access is selected,
Arc needs a per-server PIN input and login support before that model can ship. Arc currently
accepts a key and sends `apikey` alone; it does not implement subscriber PIN authentication.

No provider registration, agreement or external contact has been made by the agent. #65 was
closed administratively; the owner's distribution/access decision remains open on #478. That
closure is not evidence of provider approval.
