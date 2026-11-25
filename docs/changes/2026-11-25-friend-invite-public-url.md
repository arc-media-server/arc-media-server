# Use the public server URL for friend invites

New and regenerated friend invites use the configured Public server URL. When the setting
is empty, Arc uses the address open in the browser. If the settings request fails, the
dialog offers Retry before showing a link. Existing invites and token rules still apply.

A browser regression covers creating and regenerating a link against a scratch server.
The dialog adds no logs and records no invite links or tokens.
