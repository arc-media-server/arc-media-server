# Bundle official metadata-provider attribution (#64)

The existing credits fell back to text because no provider logos were shipped. Bundle the
original TMDB approved blue short SVG and TheTVDB's two official PNGs, with source URLs and
SHA-256 hashes. Static imports fail the build if assets are removed. About/Credits is available
to every profile; matched movie/show footers retain provider-specific details/episode credits.

Marks are contained within small boxes without alteration; TheTVDB's supplied variants follow
Arc's explicit profile theme or system theme. Attribution links go to provider homepages per
TMDB's guidance. Keep the exact TMDB non-endorsement notice and remove the extra sentence
implying every library/episode uses TMDB. No remote logo request, runtime API or schema change.

Existing focused browser checks now require loaded logos, homepage links, desktop/phone layout,
explicit-theme precedence, and ordinary-profile credits. Actual checks/limitations are in the
PR. These fixture checks do not certify all native clients or a provider distribution licence;
TheTVDB's distribution/access model remains undecided on #80. Do not close that decision.

Logging: static bundled assets introduce no server operation or failure/stall boundary, and
add no log volume. Build resolution catches missing assets; existing request/page diagnostics
cover delivery failures. No keys, credentials, owner media or production services used.
