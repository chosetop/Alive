package entry

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidSayingMeta = errors.New("entry: invalid saying meta")
	ErrInvalidSayingBody = errors.New("entry: invalid saying body")
	ErrSayingLongForm    = errors.New("entry: saying is long form")
)

type SayingMeta struct {
	Source string `json:"source,omitempty"`
	Author string `json:"author,omitempty"`
}

type SayingDraftAssessment struct{ LongFormWarning bool }

func DecodeSayingMeta(raw Meta) (SayingMeta, error) {
	var meta SayingMeta
	dec := json.NewDecoder(bytes.NewReader(raw.ForStorage()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&meta); err != nil {
		return SayingMeta{}, ErrInvalidSayingMeta
	}
	if strings.TrimSpace(meta.Source) != meta.Source || strings.TrimSpace(meta.Author) != meta.Author || utf8.RuneCountInString(meta.Source) > 200 || utf8.RuneCountInString(meta.Author) > 200 {
		return SayingMeta{}, ErrInvalidSayingMeta
	}
	return meta, nil
}

func ValidateSayingBody(body string) (SayingDraftAssessment, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return SayingDraftAssessment{}, ErrEmptyContent
	}
	for _, marker := range []string{"# ", "```", "\n- ", "\n* ", "> ", "|", "<img", "<table"} {
		if strings.Contains(body, marker) {
			return SayingDraftAssessment{}, ErrInvalidSayingBody
		}
	}
	return SayingDraftAssessment{LongFormWarning: utf8.RuneCountInString(trimmed) > 300}, nil
}

func ValidateSayingForPublish(e Entry) error {
	if _, err := ValidateSayingBody(e.ContentMD); err != nil {
		return err
	}
	if _, err := DecodeSayingMeta(e.Meta); err != nil {
		return err
	}
	if !e.Visibility.Valid() {
		return ErrInvalidVisibility
	}
	if e.Slug == "" {
		return ErrInvalidSlug
	}
	return nil
}

const sayingIDAlphabet = "23456789abcdefghjkmnpqrstuvwxyz"

type ShortIDGenerator func() (string, error)

func GenerateSayingShortID(random io.Reader) (string, error) {
	if random == nil {
		random = rand.Reader
	}
	buf := make([]byte, 10)
	for i := range buf {
		var b [1]byte
		if _, err := io.ReadFull(random, b[:]); err != nil {
			return "", err
		}
		buf[i] = sayingIDAlphabet[int(b[0])%len(sayingIDAlphabet)]
	}
	return string(buf), nil
}
