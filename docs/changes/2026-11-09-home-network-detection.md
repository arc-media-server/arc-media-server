# #350 / owner #80: detect the real home LAN, not every private peer

A regression reproduced home status for an unrelated private peer outside all server interfaces.
Replace the blanket private/link-local test with private interface subnets plus explicit LAN
Networks, retaining loopback. On Linux inspect native link metadata: do not infer home from veth,
Docker/VM bridges or tunnels; unknown link metadata fails closed. Host networking and macvlan/ipvlan
LAN interfaces retain automatic detection. Never use a caller's Host or DNS to classify clients.

Exclude discovered default gateway addresses even from explicit home CIDRs. Linux reads IPv4/IPv6
routes; Windows uses native adapter gateway enumeration. Other platforms still detect interface
subnets but do not discover gateways; NAT safety there needs separate platform validation/support.
If NAT hides clients, Arc cannot recover their real addresses: use host/br0/custom LAN networking,
or a configured trusted proxy plus the real client subnet. Adding the bridge subnet itself is not
a supported way to grant password-free internet access. Universal-CIDR rejection is in PR #352.

Settings adds automatic_home_networks (array) and home_network_bridge (boolean), both additive,
and explains detected/undetected networks and bridge limits. Keep setup/Settings' existing password
choice. Discovery is a startup snapshot; restart after changing network interfaces/routes. No media
writes, key/URL/body logs, new per-request logs or runtime DNS lookups. Existing auth outcomes remain.

Evidence: real private-peer red/green regression; deterministic host/macvlan/veth/unknown link and
proxy fixtures; IPv4/IPv6 route parsing; Windows compile only. Current Linux fixtures cannot prove
physical Unraid, host/br0 deployment or Windows runtime acceptance. Browser warning presentation is
explicitly fixture-simulated, not a claim the cloud runner uses Docker bridge networking. App tsc/build
and four affected Chromium journeys passed; e2e type-check retains the separately reported TS6133
watch-together unused-page error, deliberately reserved for the final #350 fix.
