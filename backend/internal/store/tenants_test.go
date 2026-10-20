package store

import "testing"

func TestNormalizeGSTIN(t *testing.T) {
	ok, err := NormalizeGSTIN(" 32abcde1234f1z5 ")
	if err != nil {
		t.Fatalf("valid GSTIN: %v", err)
	}
	if ok != "32ABCDE1234F1Z5" {
		t.Errorf("got %q", ok)
	}
	blank, err := NormalizeGSTIN("  ")
	if err != nil || blank != "" {
		t.Errorf("blank = %q, %v", blank, err)
	}
	if _, err := NormalizeGSTIN("32ABCDE1234"); err == nil {
		t.Fatal("expected error for short GSTIN")
	}
}

func TestNormalizeGSTPercent(t *testing.T) {
	got, err := NormalizeGSTPercent(18)
	if err != nil || got != 18 {
		t.Fatalf("18 = %v, %v", got, err)
	}
	got, err = NormalizeGSTPercent(5.555)
	if err != nil || got != 5.56 {
		t.Fatalf("round = %v, %v", got, err)
	}
	if _, err := NormalizeGSTPercent(101); err == nil {
		t.Fatal("expected error for 101")
	}
	if _, err := NormalizeGSTPercent(-1); err == nil {
		t.Fatal("expected error for -1")
	}
}
