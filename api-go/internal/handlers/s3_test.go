package handlers

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDecodeDeleteObjectsRequestParsesQuietAndObjects(t *testing.T) {
	t.Parallel()

	body := `<Delete xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Quiet>true</Quiet><Object><Key>a.txt</Key><VersionId>v1</VersionId></Object><Ignored><Nested>value</Nested></Ignored></Delete>`
	req, err := decodeDeleteObjectsRequest(strings.NewReader(body))
	if err != nil {
		t.Fatalf("decode DeleteObjects request: %v", err)
	}
	if !req.Quiet {
		t.Fatal("expected quiet mode")
	}
	if len(req.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(req.Objects))
	}
	if req.Objects[0].Key != "a.txt" || req.Objects[0].VersionID != "v1" {
		t.Fatalf("unexpected object: %+v", req.Objects[0])
	}
}

func TestDecodeDeleteObjectsRequestRejectsTooManyObjectsDuringParse(t *testing.T) {
	t.Parallel()

	var body strings.Builder
	body.WriteString("<Delete>")
	for i := 0; i < maxDeleteObjects+1; i++ {
		fmt.Fprintf(&body, "<Object><Key>too-many-%d</Key></Object>", i)
	}
	body.WriteString("</Delete>")

	req, err := decodeDeleteObjectsRequest(strings.NewReader(body.String()))
	if !errors.Is(err, errDeleteObjectsTooMany) {
		t.Fatalf("expected too many objects error, got %v", err)
	}
	if len(req.Objects) != maxDeleteObjects {
		t.Fatalf("expected parser to retain only %d objects, got %d", maxDeleteObjects, len(req.Objects))
	}
}

func TestDecodeDeleteObjectsRequestRejectsMalformedRoot(t *testing.T) {
	t.Parallel()

	_, err := decodeDeleteObjectsRequest(strings.NewReader("<NotDelete></NotDelete>"))
	if !errors.Is(err, errDeleteObjectsMalformedXML) {
		t.Fatalf("expected malformed XML error, got %v", err)
	}
}
