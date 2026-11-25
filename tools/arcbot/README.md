# ArcBot

ArcBot answers `/install`, `/report`, and `/faq`. Members use
`/tester notifications:True` or `/tester notifications:False` to opt in or out of build
announcements. Replies are private to the person who ran the command.

## First-time setup

Use the existing ArcBot application and the Arc Discord Server ID. In Discord, enable
**User Settings → Advanced → Developer Mode**, then right-click the server icon and choose
**Copy Server ID**. Keep the bot's token private.

Install Python 3.10 or newer, then run this from the repository root in PowerShell:

```powershell
py -3 -m pip install -r .\tools\arcbot\requirements.txt
$ArcGuildId = Read-Host 'Arc Discord Server ID'
py -3 .\tools\arcbot\bot.py --guild-id $ArcGuildId --configure --enable-spam
py -3 .\tools\arcbot\bot.py --guild-id $ArcGuildId
```

Both bot commands prompt for its token without echoing it. The first bot command creates a
permission-free **Tester** role, pins the reporting guide in the existing **#testing** channel
and enables Discord's built-in spam rule if the server has no spam rule. Remove
`--enable-spam` if you prefer to manage AutoMod in Discord's server settings. The last command
registers the slash commands in the Arc server and keeps ArcBot online while the process runs.

ArcBot needs **Manage Roles** to handle Tester membership, and its role must sit above
**Tester**. For first-time setup it also needs **View Channel**, **Send Messages**,
**Read Message History** and **Pin Messages** in **#testing**. Enabling AutoMod additionally
requires **Manage Server**, which you can remove after setup. Leave the Tester role without
extra permissions or channel access. An owner or moderator can mention Tester when posting
a build announcement.

To keep the slash commands available, run the last command on an always-on machine.
Provide the token as the `ARC_DISCORD_BOT_TOKEN` environment secret on that machine. On
Linux, use `python3` in place of `py -3` and forward slashes in the paths.
