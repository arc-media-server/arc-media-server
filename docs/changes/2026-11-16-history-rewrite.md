# History rewritten once before going public

The owner's checklist for making a repository public (`docs/ALPHA-CHECKLIST.md`, 13 November)
asks for agent session links to be removed from history once, just before it goes public. This
was done for the server repository on 16 November 2026.

- **Scan first.** Every version of every file on every branch (795 commits) was checked for
  secrets, personal names, emails and paths, private addresses and files from real media or
  settings. Nothing was found: the only key-like values are test placeholders named
  `fixture-key-not-a-secret`, the addresses are examples, and every author uses a GitHub
  noreply address.
- **What changed.** `git filter-repo --message-callback` removed the `Claude-Session:` trailer
  from 261 commit messages. Nothing else: every rewritten commit has exactly the same files,
  author, committer and dates as before (checked commit by commit), and `cherry picked from
  commit` lines were updated to the new commit ids.
- **References.** This commit updates 339 references to old commit ids in 43 documentation
  files to the new ids (same length). Image names that embed an old commit id (for example
  `arc-media-server:2a7029a…` in `.github/workflows/docker.yml`) are left as they are, since those
  images were published under the old id.
- **Limits.** GitHub keeps each pull request's original commits, so a PR's Commits tab can still
  show the old messages; GitHub Support can remove those on request. Issue and PR comments that
  quote old ids are not rewritten. Everyone re-clones afterwards (owner PCs, agent sessions,
  Codex), and the container image is rebuilt from the new `main`.
