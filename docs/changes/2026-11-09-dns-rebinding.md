# #350: do not grant location-based authority to unknown DNS names

Reproduced the review's DNS-rebinding case before the fix: the same attacker-controlled Origin
and Host offered the home bypass and could choose a profile. Setup also reached its handler.

Bound password-free profile selection (including existing local sessions) and first-run setup to
an explicit class of server addresses: literal IPs, single-label intranet names, reserved `.local`
and `.home.arpa` names, the machine's hostname, or the configured HTTPS domain. A trusted reverse
proxy's external Host is checked; untrusted forwarding metadata cannot expand the list. No DNS
lookup is performed, since an attacker's rebinding name would also resolve to this server.

This allow-list grants no home status: the peer/client must independently pass the home-network
check. Ordinary password/Bearer-authenticated access through other custom DNS names remains
available. Setup through an unrecognised custom name must use the server IP/local hostname.
This avoids requiring an extra setting merely to sign in with a password. No API fields removed.
The separate #350 NAT/home-network finding still needs its own fix; this does not claim NAT safety.

Regressions cover the reported profile leak/selection, setup denial, local addresses, configured
external domains, deceptive suffixes and trusted/untrusted proxy metadata. No additional logs are
needed for these expected validation denials; no caller-supplied Host, URL or credential is logged.
