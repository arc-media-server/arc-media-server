# Exact provider lookup and bounded metadata batch reads

Added profile-scoped indexed TMDB/TVDB/IMDb lookup and 100-item detail batches. Single/batch detail
hydration shares one implementation. Unknown/restricted records have identical empty/omitted
semantics; duplicate inputs/order are retained and editions/libraries are not collapsed. Offline
identities remain addressable. No provider/media work or success logging. Claude/automation contract:
docs/ITEM-LOOKUP-API.md. All schema/response changes are additive; migration 0063 adds lookup indexes.

Validation is recorded in the PR; only temporary stores/generated records/local fixtures are used.
