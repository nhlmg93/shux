package shux

import "testing"

func FuzzParseStatFields(f *testing.F) {
	f.Add("123 (sh) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22")
	f.Add("456 (cmd with spaces) R 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22")
	f.Add("")
	f.Add("no-parens fields here")

	f.Fuzz(func(t *testing.T, data string) {
		fields := parseStatFields(data)
		if fields == nil {
			t.Fatal("parseStatFields returned nil")
		}
		_ = splitFields(data)
	})
}
