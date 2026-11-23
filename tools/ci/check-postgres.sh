#!/usr/bin/env bash
# Disposable PostgreSQL tests for the manual and tagged-release verification.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

image=${1:-postgres:19.0-bookworm}
container="arc-postgres-check-${BASHPID}"
docker run --rm -d --name "$container" -p 127.0.0.1::5432 \
  --tmpfs /arc-fixture:rw,size=256m,mode=1777 -e PGDATA=/arc-fixture/data \
  -e POSTGRES_PASSWORD=arc-ci-local-owner "$image" >/dev/null
trap 'docker stop "$container" >/dev/null 2>&1 || true' EXIT
for attempt in {1..30}; do
  if docker exec "$container" pg_isready -U postgres -h 127.0.0.1 >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == 30 ]]; then
    echo 'PostgreSQL fixture did not become ready' >&2
    exit 1
  fi
  sleep 1
done
docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U postgres \
  -c "CREATE ROLE arc_fixture LOGIN PASSWORD 'arc-ci-local-fixture'" >/dev/null
for database in arc_runtime arc_upgrade arc_restore arc_onboarding; do
  docker exec "$container" createdb -U postgres -O arc_fixture "$database"
done
address=$(docker port "$container" 5432/tcp)
endpoint="postgres://arc_fixture:arc-ci-local-fixture@$address"
ARC_TEST_POSTGRES="$endpoint/arc_runtime?sslmode=disable" \
ARC_TEST_POSTGRES_UPGRADE="$endpoint/arc_upgrade?sslmode=disable" \
ARC_TEST_POSTGRES_RESTORE="$endpoint/arc_restore?sslmode=disable" \
ARC_TEST_POSTGRES_ONBOARDING="$endpoint/arc_onboarding?sslmode=disable" \
  go test -race ./internal/store ./cmd/ams -run '^TestPostgres' -count=1 -timeout 3m

docker exec "$container" psql -X -v ON_ERROR_STOP=1 -U postgres \
  -c "CREATE ROLE arc_parity LOGIN CREATEDB PASSWORD 'arc-ci-local-parity'" >/dev/null
docker exec "$container" createdb -U postgres -O arc_parity arc_parity_control
# These tests also run with SQLite in the default Go suite.
parity_tests=(
  TestWatchState
  TestRatingsAndWatchlist
  TestWatchHistoryRowsMatchTheirOwnQueries
  TestFavouriteGenresKeepEpisodeWeightsAndProfileScope
  TestRecommendedQueries
  TestUnstartedAndWatchedEpisodes
  TestRecommendationWindowsPreserveWatchOrderAccessAndBoundaries
  TestWatchedWindowsFallBackWithoutChangingResults
  TestRecentlyWatchedSeriesReadsNewestFirst
  TestBrowseDirectionPreservesFiltersAndPaging
  TestBrowseLetterJumpWholeFilteredList
  TestListItemFilters
  TestLabels
  TestMediaFiltersSameFileStreamAndSeriesAccess
  TestMediaFacetsCountTitlesAndRespectAccess
  TestMediaFacetsCountShowsOnceThroughEpisodes
  TestFacetStreamPassKeepsOwnFilesAndEpisodeRestrictions
  TestUnmatchedFilter
  TestTVRescanPreservesMultiEpisodeLinksAndCustomOrder
  TestMovieVersionsLocalAssetsAndRescan
  TestTargetedMovieScanDoesNotSweepOtherFolders
  TestTargetedTVScanPreservesShowIdentity
  TestRenamingMovieOrSeriesKeepsIdentityAndWatchState
  TestAnalysisDrainsMoreThanOnePageWithoutScanning
  TestRunNowBoundedToExistingBacklogAndDisabledTasks
  TestAnalysisActivityIdentifiesLibraryAndFile
  TestCompletionRestartAndRevisionChanges
  TestCollections
  TestEditedCollectionSurvivesRefresh
  TestImportedCopyOfAutomaticCollectionShowsOnce
  TestFolderCollections
  TestCollectionCreateRollsBackWhenInitialMemberWriteFails
  TestCollectionCreateRejectsWrongLibraryAndEpisodeBeforeWrite
  TestPlaylists
  TestPlayQueueOperationsAndConflictBoundaries
  TestPlayQueueConcurrentEditsPersistenceAndRevocation
  TestPlayQueueExpiryLimitsAndExactReorder
  TestPlayQueueReferencesSurviveEpisodeCleanupAndUnmatch
  TestMergeAndSplit
  TestMergeSeries
  TestMergeAndSplitKeepEarlierArrival
  TestMovieMergePreservesWatchedStateAndQueueIdentity
  TestEpisodeCleanupPreservesReferences
  TestCreateCollectionValidatesMembersBeforeCreatingAndKeepsOptions
  TestSmartCollectionAndPlaylistEvaluateSameRules
  TestPlayQueueAPIIsolationSnapshotAndAccessChanges
  TestDownloadStoreIdempotencyOwnershipLimitAndRevocation
  TestDownloadConcurrentAdmissionIsIdempotent
  TestDownloadConcurrentAdmissionCannotExceedDeviceLimit
  TestDownloadWorkerPauseCancelRestartAndDelete
  TestDownloadPendingRecordSurvivesDatabaseReopen
  TestDownloadPreparationRechecksRevocationWhilePaused
  TestBundleCleanupRespectsReadersAndForeignKeyCascade
  TestDownloadManifestAndSingleRangeDelivery
  TestActiveDownloadRevocationAbortsHTTPStream
  TestPlexImport
  TestJellyfinImport
  TestPlexLists
  TestJellyfinLists
  TestListsFailureKeepsWatchState
  TestVerifyPostgresConnectionRejectsDatabaseCopy
)
parity=$(IFS="|"; printf '^(%s)$' "${parity_tests[*]}")
ARC_TEST_POSTGRES_PARITY="postgres://arc_parity:arc-ci-local-parity@$address/arc_parity_control?sslmode=disable" \
  go test -race ./internal/store ./internal/scanner ./internal/background ./internal/api ./internal/downloads ./internal/importer -run "$parity" -count=1 -timeout 3m
