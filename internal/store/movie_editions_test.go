package store

import "testing"

func TestReconcileMovieEditionsPreservesOriginalHistory(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	family := "title:movie:2008"
	id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Movie", Year: 2008}, family)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE items SET movie_edition=NULL WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{"Movie (2008) {edition-IMAX} - 2160p.mkv", "Movie (2008) {edition-IMAX} - 1080p.mkv", "Movie (2008).mkv"} {
		file, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + filename, Size: 1, ModTime: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetFileItems(ctx, file, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.db.ExecContext(ctx, `INSERT INTO item_labels(item_id,label) VALUES (?,'Restricted')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, viewer.ID, id, 120000, 6000000); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := st.ReconcileMovieEditions(ctx, lib); err != nil {
			t.Fatal(err)
		}
	}
	original, err := st.GetItem(ctx, id)
	if err != nil || original == nil || original.Edition != "" {
		t.Fatalf("original=%+v err=%v", original, err)
	}
	others, err := st.OtherMovieEditions(ctx, *original)
	if err != nil || len(others) != 1 || others[0].Edition != "IMAX" {
		t.Fatalf("other editions=%+v err=%v", others, err)
	}
	cut := others[0].ID
	files, err := st.ItemFiles(ctx, cut)
	if err != nil || len(files) != 2 {
		t.Fatalf("IMAX files=%+v err=%v", files, err)
	}
	labels, err := st.ItemLabels(ctx, cut)
	if err != nil || len(labels) != 1 || labels[0] != "Restricted" {
		t.Fatalf("IMAX labels=%v err=%v", labels, err)
	}
	data, err := st.UserDataFor(ctx, viewer.ID, []string{id, cut})
	if err != nil || data[id].PositionMS != 120000 || data[cut].PositionMS != 0 {
		t.Fatalf("resume=%+v err=%v", data, err)
	}
}
