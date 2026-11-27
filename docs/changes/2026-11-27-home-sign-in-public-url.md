# Home sign-in through the Public server URL

Arc recognises the exact hostname in the saved **Public server URL** for **Skip the password
at home**. This supports split DNS and reverse proxies such as Nginx Proxy Manager, including
when Arc's own HTTPS is off. The owner can use the same hostname on the home network and share
it with friends, who still sign in when Arc sees them outside Home networks.

The existing Home network requirement, trusted proxy settings and cross-origin restrictions
still apply. Arc uses the saved hostname without a DNS lookup. This change adds no settings,
schema changes or logging. Initial setup retains its existing hostname check.
