# Collection editor: "Library default" wording

**What changed:** in a collection's editor, the "In the library" choice that follows the library's
setting now reads "Library default (show its titles)" instead of "As the library is set (show its
titles)", and likewise for the other two settings.

**Why:** the owner asked for it to match the other inherited choices, such as a show's "Library
default (Aired order)".

**Checks:** `cd web && npx tsc -b` and the production build. Only English catalog text changed.

**Logging:** none needed (text only).
