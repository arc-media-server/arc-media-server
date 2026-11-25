# Link the Arc Discord and add ArcBot

The server README links to the project website and the Arc Discord. ArcBot runs separately
and answers common setup questions, links to installation and issue reporting, and lets members
opt in to testing notices with a permission-free Tester role.

A one-time setup command can create the Tester role, pin a testing guide, and enable Discord's
built-in spam filter. It preserves any spam rule already configured on the Discord server.

Validation covers command metadata, replies, membership changes and repeated setup using a local
Discord API simulation. Live Discord validation requires the installed bot's credential and the
appropriate server permissions. ArcBot logs command registration and role update failures without
recording interaction contents or credentials.
