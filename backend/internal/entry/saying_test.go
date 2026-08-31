package entry

import (
	"bytes"
	"regexp"
	"testing"
)

func TestGenerateSayingShortID(t *testing.T) {
	id, err := GenerateSayingShortID(bytes.NewReader(bytes.Repeat([]byte{1}, 10)))
	if err != nil || len(id) != 10 {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if !regexp.MustCompile(`^[23456789abcdefghjkmnpqrstuvwxyz]{10}$`).MatchString(id) {
		t.Fatal(id)
	}
}

func TestValidateSayingBody(t *testing.T) {
	assessment, err := ValidateSayingBody("第一行\n第二行")
	if err != nil || assessment.LongFormWarning {
		t.Fatalf("assessment=%+v err=%v", assessment, err)
	}
	assessment, err = ValidateSayingBody(string(bytes.Repeat([]byte("中"), 301)))
	if err != nil || !assessment.LongFormWarning {
		t.Fatalf("assessment=%+v err=%v", assessment, err)
	}
	if _, err := ValidateSayingBody("# heading"); err != ErrInvalidSayingBody {
		t.Fatalf("err=%v", err)
	}
}

func TestDecodeSayingMetaRejectsUnknownFields(t *testing.T) {
	if _, err := DecodeSayingMeta(Meta(`{"source":"x","unknown":true}`)); err != ErrInvalidSayingMeta {
		t.Fatalf("err=%v", err)
	}
}
