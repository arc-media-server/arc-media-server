package store

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestLibrarySettingsTransactionRollsBackCoreAndFiles(t *testing.T) {
	st, err := Open(t.Context(), filepath.Join(t.TempDir(), "fixture.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	root := t.TempDir()
	id, err := st.SyncLibrary(t.Context(), "Before", "movies", "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	file := MediaFile{LibraryID: id, Path: filepath.Join(root, "movie.mkv"), Size: 1, ModTime: 1}
	file.ID, err = st.SaveFile(t.Context(), &file, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	before, err := st.GetLibrary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.db.ExecContext(t.Context(), `CREATE TRIGGER fixture_reject_options BEFORE UPDATE OF collection_min ON libraries WHEN NEW.collection_min=7 BEGIN SELECT RAISE(ABORT,'fixture option failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	after := *before
	after.Name = "After"
	after.Paths = []string{t.TempDir()}
	after.Trickplay = true
	after.CollectionMin = 7
	if err := st.SaveLibrarySettings(t.Context(), *before, after, true); err == nil {
		t.Fatal("injected failure accepted")
	}
	saved, err := st.GetLibrary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, saved) {
		t.Fatalf("partially saved after rollback: %+v", saved)
	}
	kept, err := st.GetFileByPath(t.Context(), file.Path)
	if err != nil || kept == nil || kept.ID != file.ID {
		t.Fatalf("file deletion escaped rollback: %+v %v", kept, err)
	}
}
