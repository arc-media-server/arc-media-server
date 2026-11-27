"""Arc's Discord community bot, run independently of Arc Media Server."""

import argparse
import asyncio
import getpass
import os
import sys

import discord
from discord import app_commands

WEBSITE = "https://arcmediaserver.org"
REPO = "https://github.com/arc-media-server/arc-media-server"
TESTER_ROLE = "Tester"
TESTING_GUIDE = """**Help test Arc**

Use `/tester notifications:True` to opt in to pings about new builds, or
`/tester notifications:False` to opt out.

When reporting a problem, tell us your device model, Arc app and server versions, what you
did, and what happened. Keep tokens, passwords, and private server addresses out of posts.
Use `/report` to find the right GitHub repository for a reproducible issue."""
FAQ = {
    "Connect a TV": (
        "Open the Arc app on your TV and enter the address you use to reach your Arc server. "
        "If the app shows a code, open the profile menu in Arc’s web app, choose Link a Device, "
        "and enter the code. "
        f"[Find your TV app and its instructions]({WEBSITE}/apps/)."
    ),
    "Run Arc Server": (
        "Run Arc on a computer or NAS that can read your media. The web app comes with the "
        f"server. [Docker quickstart]({REPO}#running-with-docker)."
    ),
    "Choose a database": (
        "SQLite is the default and needs no separate database server. PostgreSQL is an option "
        f"during first-time setup. [More setup answers]({WEBSITE}/support/)."
    ),
}


def find_tester_role(guild: discord.Guild, channels: list) -> discord.Role | None:
    roles = [role for role in guild.roles if role.name == TESTER_ROLE]
    if not roles:
        return None
    if len(roles) != 1:
        raise ValueError("ArcBot needs exactly one role named Tester.")
    role = roles[0]
    if role.managed or role.permissions.value:
        raise ValueError("Tester must be a standard role with no server permissions.")
    if any(channel.overwrites_for(role).pair()[0].value for channel in channels):
        raise ValueError("Tester must have no channel permissions granted to it.")
    return role


class ArcBot(discord.Client):
    def __init__(self, guild_id: int):
        super().__init__(intents=discord.Intents(guilds=True), allowed_mentions=discord.AllowedMentions.none())
        self.arc_guild = discord.Object(guild_id)
        self.tree = app_commands.CommandTree(self)
        for command in (install, report, faq, tester):
            self.tree.add_command(command, guild=self.arc_guild)

    async def setup_hook(self):
        synced = await self.tree.sync(guild=self.arc_guild)
        print(f"Registered {len(synced)} ArcBot commands.", flush=True)


@app_commands.command(description="Find the Arc install instructions for your device")
async def install(interaction: discord.Interaction):
    await interaction.response.send_message(
        f"[Apps and installation instructions]({WEBSITE}/apps/)\n"
        f"[Set up Arc Media Server with Docker]({REPO}#running-with-docker)",
        ephemeral=True,
    )


@app_commands.command(description="Find where to report an Arc bug or request a feature")
async def report(interaction: discord.Interaction):
    await interaction.response.send_message(
        f"[Server or web app issue]({REPO}/issues/new/choose)\n"
        f"[Find the repository for your TV app]({WEBSITE}/apps/)\n"
        "For a bug, include your version, device, and steps to reproduce it.",
        ephemeral=True,
    )


@app_commands.command(description="Get an answer to a common Arc setup question")
@app_commands.choices(topic=[app_commands.Choice(name=topic, value=topic) for topic in FAQ])
async def faq(interaction: discord.Interaction, topic: app_commands.Choice[str] | None = None):
    answer = FAQ[topic.value] if topic else f"[Read the Arc FAQ and setup guide]({WEBSITE}/support/)."
    await interaction.response.send_message(answer, ephemeral=True)


@app_commands.command(description="Opt in or out of Arc testing announcements")
@app_commands.describe(notifications="True to get testing announcements, False to stop them")
async def tester(interaction: discord.Interaction, notifications: bool):
    guild, member = interaction.guild, interaction.user
    if guild is None or not isinstance(member, discord.Member):
        await interaction.response.send_message("Use this in the Arc Discord server.", ephemeral=True)
        return

    await interaction.response.defer(ephemeral=True, thinking=True)
    try:
        role = find_tester_role(guild, guild.channels)
        if role is None:
            raise ValueError("Create the Tester role before enabling opt-ins.")
        if notifications:
            await member.add_roles(role, reason="Arc testing notifications opt-in")
        else:
            await member.remove_roles(role, reason="Arc testing notifications opt-out")
    except (ValueError, discord.HTTPException) as error:
        detail = str(error) if isinstance(error, ValueError) else f"Discord HTTP {error.status}"
        print(f"ArcBot tester role update failed: {detail}", file=sys.stderr, flush=True)
        await interaction.followup.send("I couldn’t update your Tester role. Ask a moderator for help.", ephemeral=True)
        return

    response = "You’re opted in to Arc testing announcements." if notifications else "You’re opted out of Arc testing announcements."
    await interaction.followup.send(response, ephemeral=True)


async def configure(token: str, guild_id: int, enable_spam: bool):
    """Set up community resources once, then exit without connecting to the Gateway."""
    async with discord.Client(intents=discord.Intents.none(), allowed_mentions=discord.AllowedMentions.none()) as client:
        await client.login(token)
        guild = await client.fetch_guild(guild_id)
        channels = await guild.fetch_channels()
        testing_channels = [channel for channel in channels if isinstance(channel, discord.TextChannel) and channel.name == "testing"]
        if len(testing_channels) != 1:
            raise ValueError("ArcBot needs exactly one text channel named #testing.")
        channel = testing_channels[0]

        if find_tester_role(guild, channels) is None:
            await guild.create_role(name=TESTER_ROLE, permissions=discord.Permissions.none())
            print("Created the Tester role.")

        async for pin in channel.pins(limit=None):
            if pin.message.author == client.user and pin.message.content == TESTING_GUIDE:
                break
        else:
            message = None
            async for recent in channel.history(limit=50):
                if recent.author == client.user and recent.content == TESTING_GUIDE:
                    message = recent
                    break
            if message is None:
                message = await channel.send(TESTING_GUIDE)
            await message.pin()
            print("Pinned the testing guide.")

        if enable_spam:
            rules = await guild.fetch_automod_rules()
            spam_rules = [rule for rule in rules if rule.trigger.type is discord.AutoModRuleTriggerType.spam]
            if spam_rules:
                print("A Discord spam rule already exists. Check its enabled status in Server Settings.")
            else:
                await guild.create_automod_rule(
                    name="Block spam",
                    event_type=discord.AutoModRuleEventType.message_send,
                    trigger=discord.AutoModTrigger(type=discord.AutoModRuleTriggerType.spam),
                    actions=[discord.AutoModRuleAction(type=discord.AutoModRuleActionType.block_message)],
                    enabled=True,
                )
                print("Enabled Discord’s built-in spam rule.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--guild-id", type=int, required=True, help="Arc Discord server ID")
    parser.add_argument("--configure", action="store_true", help="Create the Tester role and pin the testing guide")
    parser.add_argument("--enable-spam", action="store_true", help="Enable Discord AutoMod spam blocking during setup")
    args = parser.parse_args()
    if args.enable_spam and not args.configure:
        parser.error("--enable-spam requires --configure")
    token = (os.getenv("ARC_DISCORD_BOT_TOKEN") or getpass.getpass("ArcBot token (hidden): ")).strip()
    if not token:
        parser.error("an ArcBot token is required")
    try:
        if args.configure:
            asyncio.run(configure(token, args.guild_id, args.enable_spam))
        else:
            ArcBot(args.guild_id).run(token)
    except discord.LoginFailure:
        sys.exit("Discord rejected the ArcBot token.")
    except discord.HTTPException as error:
        sys.exit(f"Discord returned HTTP {error.status} (code {error.code}). Check the server ID and ArcBot’s permissions.")
    except ValueError as error:
        sys.exit(str(error))


if __name__ == "__main__":
    main()
