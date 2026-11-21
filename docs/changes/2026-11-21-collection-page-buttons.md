# Collection page: one Play button, the rest round and in More

**What changed:** a collection's page now lays out its buttons like a movie's title page:

- one wide **Play** button;
- a round **Shuffle** button;
- a round **⋯ (More)** button holding **Edit**, **Arrange** (manual collections with two or more
  titles) and **Delete** (or **Remove** for automatic and folder collections).

While arranging, a **Done** button sits beside them. Before, Edit, Arrange and Delete were a
second row of full-size buttons under Play and Shuffle.

**Why:** the owner asked for the collection page to match the title page: one main action, the
rest smaller.

**Checks:** `npx tsc -b`, the production build, and a new browser journey
(`web/e2e/collection-page.spec.ts`): the buttons, the More menu, Arrange and Done, and phone
width. Screenshots in `docs/changes/2026-11-21-collection-page-buttons/`.

**Logging:** none needed (layout only).
