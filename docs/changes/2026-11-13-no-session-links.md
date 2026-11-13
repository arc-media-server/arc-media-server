# No agent session links in commits or PRs; clean history before going public

Commits and PR descriptions made by Claude carried a `Claude-Session:` link to the chat that made
them. The links only open for the owner's account, but the owner doesn't want them in what becomes
public history. From now on the Co-Authored-By trailer is the only attribution (CLAUDE.md here and
in each app repository's notes).

The links already in history stay until the repositories go public: rewriting history means a
force push that breaks every existing clone, branch and PR, so it is done once, as a planned step,
with a secrets scan of the whole history at the same time. `docs/ALPHA-CHECKLIST.md` gains a
"Before any repository is made public" section listing the steps.
