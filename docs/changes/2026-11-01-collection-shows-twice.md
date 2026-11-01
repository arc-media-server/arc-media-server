# A collection no longer shows twice

**What changed.** Some movies showed the same collection row twice on their page (on the web and
on the TV apps), and the library's Collections tab could show two identical tiles. The cause is
a Plex import: Plex keeps its own copy of each TMDB collection ("Toy Story Collection"), the
import brings it over as a manual collection, and Arc also makes the same collection
automatically from TMDB. The title was in both, so both rows showed.

- A title's page shows one row when the title is in a manual collection and an automatic one of
  the same name (case and spaces ignored): the manual one, which keeps the imported order and
  any edits.
- The library's collections leave out the automatic one while the manual one holds every one of
  its titles. If TMDB's collection gains a film the manual copy lacks, both show again, so that
  film isn't hidden (nothing silently disappears).

**Why here.** The fix is in the server (`store.Collections` and `store.ItemCollections`), so the
web, Android and LG/Samsung apps all get it with no app change, and existing libraries are fixed
without touching their data. Nothing is deleted or merged in the database; removing the manual
copy brings the automatic one back.

**Limits.** Only exact name matches are treated as the same collection. A manual collection
renamed differently from TMDB's shows separately, as before.

**Logging.** None added: this is a listing rule, and its outcome is visible in the UI. The
existing request logging covers failures of the collection queries.
