package resources

import (
	"testing"
	"testing/fstest"
)

func TestLogoParses(t *testing.T) {
	if Logo() == nil {
		t.Fatal("the app logo does not parse")
	}
}

func TestBundledFontsExist(t *testing.T) {
	entries, err := fonts.ReadDir(fontDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("bundled fonts = %v, %v", entries, err)
	}
}

func TestRegisterFontsRejectsBadData(t *testing.T) {
	bad := fstest.MapFS{fontDir + "/Broken" + fontExt: {Data: []byte("not a font")}}
	if err := registerFonts(bad); err == nil {
		t.Fatal("broken font data registered without an error")
	}
}
