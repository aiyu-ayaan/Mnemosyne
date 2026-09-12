package install

import "testing"

const dir = `C:\Users\me\AppData\Local\Programs\Mnemosyne`

func TestAddEntryAppends(t *testing.T) {
	got, changed := addEntry(`C:\Windows;C:\Windows\System32`, dir)
	if !changed {
		t.Fatal("changed = false, want true")
	}
	want := `C:\Windows;C:\Windows\System32;` + dir
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestAddEntryIsIdempotent(t *testing.T) {
	// Installing twice must not grow PATH.
	once, _ := addEntry(`C:\Windows`, dir)
	twice, changed := addEntry(once, dir)

	if changed {
		t.Error("changed = true on the second add; PATH would grow on every install")
	}
	if twice != once {
		t.Errorf("PATH changed on the second add:\n%q\n%q", once, twice)
	}
}

func TestAddEntryIgnoresCaseAndTrailingSlash(t *testing.T) {
	existing := `C:\Windows;c:\users\me\appdata\local\programs\mnemosyne\`
	if _, changed := addEntry(existing, dir); changed {
		t.Error("added a duplicate that differed only in case and a trailing slash")
	}
}

func TestAddEntryOnEmptyPath(t *testing.T) {
	got, changed := addEntry("", dir)
	if !changed || got != dir {
		t.Errorf("addEntry(\"\") = %q, %v", got, changed)
	}
}

func TestAddEntryHandlesTrailingSeparator(t *testing.T) {
	got, _ := addEntry(`C:\Windows;`, dir)
	want := `C:\Windows;` + dir
	if got != want {
		t.Errorf("got %q, want %q (no empty entry from the trailing separator)", got, want)
	}
}

func TestRemoveEntry(t *testing.T) {
	got, changed := removeEntry(`C:\Windows;`+dir+`;C:\Other`, dir)
	if !changed {
		t.Fatal("changed = false, want true")
	}
	want := `C:\Windows;C:\Other`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRemoveEntryLeavesOtherPathsAlone(t *testing.T) {
	original := `C:\Windows;C:\Other`
	got, changed := removeEntry(original, dir)
	if changed {
		t.Error("changed = true when the entry was not present")
	}
	if got != original {
		t.Errorf("PATH was rewritten despite no match:\n%q\n%q", original, got)
	}
}

func TestRemoveEntryDropsEveryCopy(t *testing.T) {
	got, _ := removeEntry(dir+`;C:\Windows;`+dir, dir)
	if got != `C:\Windows` {
		t.Errorf("got %q, want a PATH with no copies left", got)
	}
}

func TestRemoveEntryDoesNotMatchAPrefix(t *testing.T) {
	// A directory whose name merely starts the same must survive.
	sibling := dir + `Extra`
	got, _ := removeEntry(sibling+`;`+dir, dir)
	if got != sibling {
		t.Errorf("got %q, want %q kept", got, sibling)
	}
}
